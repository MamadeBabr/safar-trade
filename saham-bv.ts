import { Logger } from '@chartix/logging';
import { NatsClient } from '@chartix/nats-client';
import { OrchestratorClient } from '@chartix/orchestrator-client';
import type {
  TickMessage,
  FinalPriceMessage,
  TsetmcDayOffRequestMessage,
  TsetmcDayOffResponseMessage,
} from '@chartix/shared-types';
import { AppType, chunkArray, defineApp, isTsetmcTimestampInTimeRange } from '@chartix/shared-utils';
import * as schedule from 'node-schedule';
import { WebSocket } from 'ws';
import {
  getWatchedSymbols,
  startSymbolWatch,
  stopSymbolWatch,
  type WatchedSymbol,
} from './symbol-watch.js';
import type { ClosingInstrumentData, HistoryInstrumentData } from './types/InstrumentData.type';

const logger = new Logger('Bourse View Live Price');

interface ManagedSocket {
  ws: WebSocket;
  name: string;
  namadSlice: string[];
}

// State Management
const allSymbols: Set<string> = new Set();
const symbolMap: Map<string, { symbol: string; type: number }> = new Map();
let activeSockets: ManagedSocket[] = [];
/** Instrument id → the socket it is subscribed on (for incremental unsubscribes). */
const instrumentToSocket = new Map<string, ManagedSocket>();
/** True once a session's sockets are up — gates incremental wiring of symbol changes. */
let connectionsLive = false;
/** Monotonic socket-name counter (names only need to be unique per process). */
let socketCounter = 0;

// Configuration
const MARKET_OPEN_HOUR = 8;
const MARKET_OPEN_MINUTE = 50;
const MARKET_CLOSE_HOUR = 17;
const MARKET_CLOSE_MINUTE = 10;
const MARKET_TIMEZONE = 'Asia/Tehran';
const MAX_SYMBOLS_PER_SOCKET = 500;
const RETRY_DAYOFF_INTERVAL_MS = 3 * 60 * 1_000;
const RECONNECT_DELAY_MS = 10_000;

let openJob: schedule.Job | undefined;
let closeJob: schedule.Job | undefined;

const instrumentVolumeMap = new Map<string, number>([
  ["IRTKNGRI0001", 0]
]);

const closingPriceMap = new Map<string, number>();

function isMarketHours(): boolean {
  const now = new Date().toLocaleString('en-US', { timeZone: MARKET_TIMEZONE });
  const date = new Date(now);
  const currentMinutes = date.getHours() * 60 + date.getMinutes();
  const startMinutes = MARKET_OPEN_HOUR * 60 + MARKET_OPEN_MINUTE;
  const endMinutes = MARKET_CLOSE_HOUR * 60 + MARKET_CLOSE_MINUTE;
  return currentMinutes >= startMinutes && currentMinutes < endMinutes;
}

async function waitForDayOffCheck(): Promise<boolean | null> {
  while (isMarketHours()) {
    try {
      const result = await NatsClient.request<TsetmcDayOffResponseMessage, TsetmcDayOffRequestMessage>(
        'landing.tsetmc.dayoff.read',
        {},
      );
      if (result.success && result.data?.data) {
        return result.data.data.isDayOff;
      }
    } catch (error) {
      logger.warn(`Day-off check failed: ${(error as Error).message} — retrying in 3 minutes`);
    }
    await new Promise(resolve => setTimeout(resolve, RETRY_DAYOFF_INTERVAL_MS));
  }
  logger.warn('Day-off check timed out — market hours ended without a successful response');
  return null;
}

function getTimeDifference(timestamp1: number, timestamp2: number) {
  return Math.abs(timestamp1 - timestamp2);
}

const headers = {
  Host: 'bourseview.emofid.com',
  Connection: 'Upgrade',
  Pragma: 'no-cache',
  'Cache-Control': 'no-cache',
  'User-Agent':
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36',
  Upgrade: 'websocket',
  Origin: 'https://bv.emofid.com',
  'Sec-WebSocket-Version': '13',
  'Accept-Encoding': 'gzip, deflate, br, zstd',
  'Accept-Language': 'en-GB,en;q=0.9',
  'Sec-WebSocket-Extensions': 'permessage-deflate; client_max_window_bits',
};

const WS_URL = `wss://bourseview.emofid.com/socket.io/?clientId=BourseViewWeb&EIO=4&transport=websocket`;

function sendSubscribe(ws: WebSocket, isin: string): void {
  ws.send(`42${JSON.stringify(['history', { cmd: 'sub', isin: isin.toUpperCase(), resolution: '1D' }])}`);
  ws.send(`42${JSON.stringify(['closing', { cmd: 'sub', isin: isin.toUpperCase() }])}`);
}

function sendUnsubscribe(ws: WebSocket, isin: string): void {
  ws.send(`42${JSON.stringify(['history', { cmd: 'unsub', isin: isin.toUpperCase(), resolution: '1D' }])}`);
  ws.send(`42${JSON.stringify(['closing', { cmd: 'unsub', isin: isin.toUpperCase() }])}`);
}

const createSocket = (
  name: string,
  namadSlice: string[],
): ManagedSocket => {
  const ws = new WebSocket(WS_URL, { headers });
  const socket: ManagedSocket = { ws, name, namadSlice };

  ws.on('open', (): void => {
    logger.info(`[${name}] WebSocket opened`);
  });

  ws.on('message', (data: Buffer): void => {
    const msg = data.toString();

    if (msg.startsWith('0') && !msg.startsWith('40') && !msg.startsWith('42')) {
      ws.send('40');
      return;
    }

    if (msg.startsWith('40')) {
      logger.info(
        `[${name}] Namespace connected! Subscribing to ${namadSlice.length} instruments...`,
      );
      
      for (const namad of namadSlice) {
        sendSubscribe(ws, namad);
      }
      return;
    }

    if (msg.startsWith('42')) {
      try {
        const parsed = JSON.parse(msg.slice(2));
        const eventName: string = parsed[0];

        if (eventName.startsWith('closing')) {
          const { instrumentId, price  , prevPrice}: ClosingInstrumentData = parsed[1];
          const symbol = symbolMap.get(instrumentId)?.symbol;
          if (!symbol) return;

          if (closingPriceMap.get(symbol) !== price) {
              
            closingPriceMap.set(symbol, price);
            
            OrchestratorClient.getOwnerFor('service', symbol, 'data-processor', 'TSETMC').then((instance) => {
                if (!instance || !price) return;
                
                const payload : FinalPriceMessage = {
                  exchange:"TSETMC" , 
                  finalPrice:price , 
                  symbol , 
                  previousFinalPrice:prevPrice
                }
            
                NatsClient.publish<FinalPriceMessage>(
                  `dataProcessor.finalPrice.${instance}.TSETMC.${symbol}`,
                  payload,
                );
              });           
          }
          return;
        }

        if (eventName.startsWith('history')) {
          const instrument: HistoryInstrumentData = parsed[1];

          const entry = symbolMap.get(instrument.instrumentId);

          if (entry) {
            if (!isTsetmcTimestampInTimeRange(instrument.lastTradeTime, entry.type)) return;

            const now = Date.now();
            const diffSeconds = getTimeDifference(now, instrument.lastTradeTime) / 1000;
            const previousVolume = instrumentVolumeMap.get(entry.symbol);
            const correctedVolume = previousVolume !== undefined ? Math.abs(instrument.volume - previousVolume) : 0;

            if (diffSeconds < 25 && diffSeconds > 0) {
              const payload: TickMessage = {
                exchange: 'TSETMC',
                symbol: entry.symbol,
                price: instrument.close,
                volume: correctedVolume,
                timestamp: instrument.lastTradeTime,
              };

              instrumentVolumeMap.set(entry.symbol, instrument.volume);

              OrchestratorClient.getOwnerFor('service', entry.symbol, 'data-processor', 'TSETMC').then((instance) => {
                if (!instance || !payload.volume) return;

                NatsClient.publish<TickMessage>(
                  `dataProcessor.tick.${instance}.TSETMC.${entry.symbol}`,
                  payload,
                );
              });
            }
          }
        }
      } catch (_e) {
        // Suppress generic parse errors to keep logs clean
      }
      return;
    }

    if (msg === '2') {
      ws.send('3');
      return;
    }
  });

  ws.on('close', (code) => {
    if (code !== 1000) {
      logger.warn(`[${name}] Closed unexpectedly: ${code}`);
    }
    if (isMarketHours()) {
      logger.info(`[${name}] Reconnecting in ${RECONNECT_DELAY_MS / 1000}s...`);
      setTimeout(() => {
        if (!isMarketHours()) return;
        const newEntry = createSocket(name, socket.namadSlice);
        const idx = activeSockets.findIndex(s => s.name === name);
        if (idx !== -1) activeSockets[idx] = newEntry;
        else activeSockets.push(newEntry);
      }, RECONNECT_DELAY_MS);
    }
  });

  ws.on('error', (err) => logger.error(`[${name}] Error: ${err.message}`));

  // Track which socket owns each instrument so removals can unsubscribe in place.
  for (const isin of namadSlice) {
    instrumentToSocket.set(isin, socket);
  }

  return socket;
};

/** Route a newly added instrument onto a socket with spare capacity, growing the pool if needed. */
function subscribeInstrument(isin: string): void {
  if (instrumentToSocket.has(isin)) return;

  let target = activeSockets.find((entry) => entry.namadSlice.length < MAX_SYMBOLS_PER_SOCKET);
  if (!target) {
    target = createSocket(`app:${socketCounter++}`, []);
    activeSockets.push(target);
  }

  target.namadSlice.push(isin);
  instrumentToSocket.set(isin, target);
  if (target.ws.readyState === WebSocket.OPEN) {
    sendSubscribe(target.ws, isin);
  }
}

/** Drop one instrument from its socket; terminate the socket when it empties out. */
function unsubscribeInstrument(isin: string): void {
  const target = instrumentToSocket.get(isin);
  if (!target) return;

  instrumentToSocket.delete(isin);
  const idx = target.namadSlice.indexOf(isin);
  if (idx !== -1) target.namadSlice.splice(idx, 1);

  if (target.ws.readyState === WebSocket.OPEN) {
    sendUnsubscribe(target.ws, isin);
  }
  if (target.namadSlice.length === 0) {
    terminateSocket(target);
  }
}

function terminateSocket(target: ManagedSocket): void {
  activeSockets = activeSockets.filter((entry) => entry !== target);
  for (const isin of target.namadSlice) {
    if (instrumentToSocket.get(isin) === target) instrumentToSocket.delete(isin);
  }
  target.ws.removeAllListeners('close');
  target.ws.terminate();
}

export function handleSymbolsAdded(symbols: WatchedSymbol[]): void {
  if (!connectionsLive) return;
  for (const entry of symbols) {
    allSymbols.add(entry.instrumentId);
    symbolMap.set(entry.instrumentId, { symbol: entry.symbol, type: entry.type });
    subscribeInstrument(entry.instrumentId);
    logger.info(`Subscribed new symbol ${entry.symbol} (${entry.instrumentId})`);
  }
}

export function handleSymbolsRemoved(symbols: WatchedSymbol[]): void {
  if (!connectionsLive) return;
  for (const entry of symbols) {
    allSymbols.delete(entry.instrumentId);
    symbolMap.delete(entry.instrumentId);
    instrumentVolumeMap.delete(entry.symbol);
    closingPriceMap.delete(entry.symbol);
    unsubscribeInstrument(entry.instrumentId);
    logger.info(`Unsubscribed removed symbol ${entry.symbol} (${entry.instrumentId})`);
  }
}

function cleanupConnections() {
  connectionsLive = false;
  if (activeSockets.length > 0) {
    logger.info(`Cleaning up ${activeSockets.length} active sockets...`);
    activeSockets.forEach((entry) => {
      entry.ws.removeAllListeners('close');
      entry.ws.terminate();
    });
    activeSockets = [];
  }
  instrumentToSocket.clear();
  allSymbols.clear();
  symbolMap.clear();
}

async function startConnections(items?: string[]): Promise<void> {
  // Subscribe to symbol changes before wiring sockets. On the first call this
  // also fetches the registry baseline; later calls are no-ops (idempotent).
  await startSymbolWatch();

  if (items) {
    allSymbols.clear();
    symbolMap.clear();
    for (const id of items) {
      allSymbols.add(id);
    }
  } else {
    // The baseline now lives in the symbol watch cache; copy the subscribable
    // (instrument id ends in `1`) entries into the wiring maps.
    for (const entry of getWatchedSymbols()) {
      if (!entry.instrumentId.endsWith('1')) continue;
      allSymbols.add(entry.instrumentId);
      symbolMap.set(entry.instrumentId, { symbol: entry.symbol, type: entry.type });
    }
    if (allSymbols.size === 0) {
      logger.warn('Symbol watch returned no subscribable symbols — starting with no sockets');
    }
  }

  const symbolArr = [...allSymbols];
  logger.info(`All Symbols length: ${symbolArr.length}`);

  const chunked = chunkArray(symbolArr, MAX_SYMBOLS_PER_SOCKET);
  for (const chunk of chunked) {
    const entry = createSocket(`app:${socketCounter++}`, chunk);
    activeSockets.push(entry);
  }

  connectionsLive = true;
  logger.info(`Cycle started with ${activeSockets.length} sockets.`);
}

async function scheduleJobs(): Promise<void> {
  openJob = schedule.scheduleJob({ hour: MARKET_OPEN_HOUR, minute: MARKET_OPEN_MINUTE, tz: MARKET_TIMEZONE }, async () => {
    logger.info('Market opens at 8:50 AM (Tehran time)');
    try {
      const dayOff = await waitForDayOffCheck();
      if (dayOff === null) {
        logger.info('Market hours ended during day-off check — skipping connection start');
        return;
      }
      if (dayOff) {
        logger.info('Today is a day off — skipping connection start');
        return;
      }
      cleanupConnections();
      await startConnections();
    } catch (error) {
      logger.error(`Open job failed: ${(error as Error).message}`);
    }
  });

  closeJob = schedule.scheduleJob({ hour: MARKET_CLOSE_HOUR, minute: MARKET_CLOSE_MINUTE, tz: MARKET_TIMEZONE }, async () => {
    logger.info('Market closes at 17:10 PM (Tehran time)');
    try {
      cleanupConnections();
    } catch (error) {
      logger.error(`Close job failed: ${(error as Error).message}`);
    }
  });

  logger.info('Scheduled jobs configured (8:50 start, 17:10 stop)');
}

async function main() {
  cleanupConnections();

  // Start watching symbol changes at boot so adds/removes during the session are
  // applied incrementally instead of restarting every socket.
  await startSymbolWatch();

  try {
    if (isMarketHours()) {
      const dayOff = await waitForDayOffCheck();
      if (dayOff === null) {
        logger.info('Market hours ended during day-off check — skipping initial connection');
      } else if (dayOff) {
        logger.info('Today is a day off — skipping initial connection');
      } else {
        await startConnections();
      }
    }

    scheduleJobs();
  } catch (error) {
    logger.error(`Critical error in main cycle: ${(error as Error).message}`);
  }
}

void defineApp({
  appName: 'BV-live-price',
  appType: AppType.COLLECTOR,
  exchange: ['TSETMC'],
  setup: main,
  maxRetries: 5,
  onDrain: async () => {
    openJob?.cancel();
    closeJob?.cancel();
    await stopSymbolWatch();
    cleanupConnections();
  },
});
