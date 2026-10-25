import { Logger } from '@chartix/logging';
import { NatsClient } from '@chartix/nats-client';
import type { NatsResponseMessage, OneMinuteCandleMessage } from '@chartix/shared-types';
import { AppType, defineApp } from '@chartix/shared-utils';
import type { Subscription } from 'nats';
import { CandleCache } from './candles/candle-cache';
import { LiveBuffer } from './candles/live-buffer';
import {
  CACHE_PUSH_INTERVAL_MS,
  FUTURES_FLUSH_INTERVAL_MS,
  FUTURES_SYMBOL_SUFFIX,
  FUTURES_WS_URL,
  SPOT_FLUSH_INTERVAL_MS,
  SPOT_WS_URL,
} from './constants';
import { env } from './env';
import { dumpCache, startCandleHttpServer } from './server';
import { ActiveSymbolsTracker } from './active-symbols';
import { BinanceStream } from './streams/binance-stream';
import { getActiveBinanceSymbols } from './symbols';
import { startSymbolRequestHandlers } from './symbol-proxy';
import type { OneMinuteCandle } from './types';

const logger = new Logger('binance-collector');

const activeSymbolsTracker = new ActiveSymbolsTracker();

let futuresStream: BinanceStream | null = null;
let spotStream: BinanceStream | null = null;
let futuresBuffer: LiveBuffer | null = null;
let spotBuffer: LiveBuffer | null = null;
let httpServer: ReturnType<typeof startCandleHttpServer> | null = null;
let cachePushTimer: NodeJS.Timeout | null = null;
let symbolRequestSubs: Subscription[] = [];

async function publishCandles(candles: OneMinuteCandle[]): Promise<void> {
  if (env.BINANCE_COLLECTOR_TRANSPORT !== 'nats') {
    logger.debug(`[publish] disabled (BINANCE_COLLECTOR_TRANSPORT=${env.BINANCE_COLLECTOR_TRANSPORT}), skipping ${candles.length} candles`);
    return;
  }

  await NatsClient.request<NatsResponseMessage, OneMinuteCandleMessage[]>(
    'binance.iran.batch',
    candles.map((c) => ({
      exchange: 'BINANCE',
      symbol: c.symbol,
      o: Number(c.open),
      h: Number(c.high),
      l: Number(c.low),
      c: Number(c.close),
      v: Number(c.volume),
      timestamp: c.openTime,
    })),
  );
}

function handleCandle(
  candle: OneMinuteCandle,
  cache: CandleCache,
  buffer: LiveBuffer | null,
): void {
  cache.update(candle);

  if (activeSymbolsTracker.isActive(candle.symbol)) {
    // Ultra-fast path: send immediately without waiting for buffer flush
    publishCandles([candle])
      .catch((err) => {
        logger.warn(`[fast-publish] failed for ${candle.symbol}:`, err);
      });
  } else {
    // Standard path: batch in buffer for periodic flush
    buffer?.push(candle);
  }
}

async function appInitializer(): Promise<void> {
  if (env.BINANCE_COLLECTOR_TRANSPORT === 'nats') {
    await activeSymbolsTracker.start();
    // Serve the symbol-data-service's upstream-cache requests (Binance
    // exchangeInfo + TradingView symbol info) — it runs in Iran and cannot reach
    // those feeds itself.
    symbolRequestSubs = await startSymbolRequestHandlers();
  }

  // ---------- Futures ----------
  const futuresSymbols = await getActiveBinanceSymbols('futures');
  logger.info(`Loaded ${futuresSymbols.length} futures symbols`);

  const futuresCache = new CandleCache();

  futuresBuffer = new LiveBuffer(async (candles) => {
    await publishCandles(candles);
    const symbolCount = new Set(candles.map((c) => c.symbol)).size;
    logger.debug(`[futures flush] ${candles.length} candles (${symbolCount} symbols) published`);
  }, FUTURES_FLUSH_INTERVAL_MS);

  futuresStream = new BinanceStream({
    url: FUTURES_WS_URL,
    market: 'futures',
    symbols: futuresSymbols,
    symbolSuffix: FUTURES_SYMBOL_SUFFIX,
    onCandle: (candle) => {
      handleCandle(candle, futuresCache, futuresBuffer);
    },
  });

  // ---------- Spot ----------
  const spotSymbols = await getActiveBinanceSymbols('spot');
  logger.info(`Loaded ${spotSymbols.length} spot symbols`);

  const spotCache = new CandleCache();

  spotBuffer = new LiveBuffer(async (candles) => {
    await publishCandles(candles);
    const symbolCount = new Set(candles.map((c) => c.symbol)).size;
    logger.debug(`[spot flush] ${candles.length} candles (${symbolCount} symbols) published`);
  }, SPOT_FLUSH_INTERVAL_MS);

  spotStream = new BinanceStream({
    url: SPOT_WS_URL,
    market: 'spot',
    symbols: spotSymbols,
    symbolSuffix: '',
    onCandle: (candle) => {
      handleCandle(candle, spotCache, spotBuffer);
    },
  });

  // ---------- HTTP read server ----------
  if (env.BINANCE_COLLECTOR_TRANSPORT === "GET") {
    httpServer = startCandleHttpServer(env.PORT, futuresCache, spotCache);
  }

  // ---------- Cache push (NATS disabled) ----------
  // When NATS publish is off, POST the candle-cache snapshot to binance-iran on
  // a fixed interval instead of relying on binance-iran polling the GET server.
  if (env.BINANCE_COLLECTOR_TRANSPORT === 'POST') {
    cachePushTimer = setInterval(() => {
      fetch(`${env.BINANCE_IRAN_POST_URL}/binance/candles`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${env.BINANCE_AUTH_TOKEN}`,
        },
        body: JSON.stringify({ futures: dumpCache(futuresCache), spot: dumpCache(spotCache) }),
      }).catch((err) => {
        logger.warn(`[cache-push] failed: ${err instanceof Error ? err.message : String(err)}`);
      });
    }, CACHE_PUSH_INTERVAL_MS);
    logger.info(`Pushing candle cache to ${env.BINANCE_IRAN_POST_URL} every ${CACHE_PUSH_INTERVAL_MS}ms`);
  }


  // ---------- Start ----------
  futuresStream.start();
  futuresBuffer.startFlushing();
  spotStream.start();
  spotBuffer.startFlushing();

  logger.info('Binance collector initialized');
}

void defineApp({
  appName: 'binance-collector',
  appType: AppType.COLLECTOR,
  exchange: ['binance'],
  setup: appInitializer,
  skipRegistration: env.BINANCE_COLLECTOR_TRANSPORT !== 'nats',
  onDrain: async () => {
    logger.info('Draining binance collector...');
    activeSymbolsTracker.stop();
    futuresStream?.stop();
    spotStream?.stop();
    futuresBuffer?.stopFlushing();
    spotBuffer?.stopFlushing();
    httpServer?.stop();
    if (cachePushTimer) clearInterval(cachePushTimer);
    for (const sub of symbolRequestSubs) {
      await sub.drain();
    }
    symbolRequestSubs = [];
  },
});
