import { Logger } from '@chartix/logging';
import { chunkArray } from '@chartix/shared-utils';
import WebSocket from 'ws';
import { MAX_STREAMS_PER_CONNECTION, RESTART_INTERVAL_MS, SUBSCRIBE_BATCH_SIZE, SUBSCRIBE_INTERVAL_MS } from '../constants.js';
import type { OneMinuteCandle, RawKlineMessage } from '../types.js';

const logger = new Logger('binance-stream');

interface BinanceStreamOptions {
  url: string;
  market: string;
  symbols: string[];
  symbolSuffix: string;
  onCandle: (candle: OneMinuteCandle) => void;
}

/**
 * One Binance market (futures or spot). Builds `<symbol>@kline_1m` stream
 * names, shards them across connections at MAX_STREAMS_PER_CONNECTION, opens a
 * `ws` per shard, batch-subscribes (SUBSCRIBE_BATCH_SIZE streams every
 * SUBSCRIBE_INTERVAL_MS to respect Binance's rate limits), parses incoming
 * `{stream, data}` kline envelopes, and auto-reconnects with exponential
 * backoff. `symbolSuffix` is appended to the candle's symbol in output
 * ("_FUTURES" for futures, "" for spot) but NOT to the WS stream name.
 */
export class BinanceStream {
  private closers: (() => void)[] = [];
  private closed = false;

  constructor(private readonly opts: BinanceStreamOptions) {}

  start(): void {
    const streams: string[] = [];
    for (const symbol of this.opts.symbols) {
      streams.push(`${symbol.toLowerCase()}@kline_1m`);
    }

    const chunks = chunkArray(streams, MAX_STREAMS_PER_CONNECTION);
    logger.info(`[${this.opts.market}] distributing ${streams.length} streams across ${chunks.length} connection(s)`);

    chunks.forEach((chunk, index) => this.openConnection(chunk, index));
  }

  stop(): void {
    this.closed = true;
    for (const close of this.closers) close();
    this.closers = [];
  }

  private openConnection(streams: string[], connIndex: number): void {
    let reconnectAttempts = 0;
    let requestId = 1;
    let subscribeQueue: string[][] = [];
    let subscribeTimer: NodeJS.Timeout | null = null;
    let ws: WebSocket | null = null;

    const restartTimer = setInterval(() => {
      logger.info(`[${this.opts.market} conn ${connIndex}] scheduled restart (${RESTART_INTERVAL_MS}ms)`);
      ws?.close();
    }, RESTART_INTERVAL_MS);

    const send = (payload: unknown): void => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(payload));
      }
    };

    const sendNextBatch = (): void => {
      if (subscribeQueue.length === 0) return;

      const batch = subscribeQueue.shift() as string[];
      send({ method: 'SUBSCRIBE', params: batch, id: requestId++ });

      if (subscribeQueue.length > 0) {
        subscribeTimer = setTimeout(sendNextBatch, SUBSCRIBE_INTERVAL_MS);
      } else {
        logger.info(`[${this.opts.market} conn ${connIndex}] all subscribe batches sent`);
      }
    };

    const connect = (): void => {
      ws = new WebSocket(this.opts.url);

      ws.on('open', () => {
        reconnectAttempts = 0;
        logger.info(`[${this.opts.market} conn ${connIndex}] connected, subscribing to ${streams.length} streams`);

        subscribeQueue = [];
        for (let i = 0; i < streams.length; i += SUBSCRIBE_BATCH_SIZE) {
          subscribeQueue.push(streams.slice(i, i + SUBSCRIBE_BATCH_SIZE));
        }
        sendNextBatch();
      });

      ws.on('message', (raw: WebSocket.RawData) => {
        let msg: { stream?: string; data?: RawKlineMessage };
        try {
          msg = JSON.parse(raw.toString());
        } catch (err) {
          logger.warn(`[${this.opts.market} conn ${connIndex}] parse-error: ${(err as Error).message}`);
          return;
        }

        if (msg.stream && msg.data !== undefined) {
          this.handleKline(msg.data);
        }
      });

      ws.on('close', (code, reason) => {
        if (subscribeTimer) {
          clearTimeout(subscribeTimer);
          subscribeTimer = null;
        }
        logger.info(`[${this.opts.market} conn ${connIndex}] closed (code ${code}) ${reason.toString()}`);
        if (!this.closed) {
          scheduleReconnect();
        }
      });

      ws.on('error', (err) => {
        logger.warn(`[${this.opts.market} conn ${connIndex}] ws-error: ${err.message}`);
      });
    };

    const scheduleReconnect = (): void => {
      reconnectAttempts++;
      const delay = Math.min(30_000, 1000 * 2 ** Math.min(reconnectAttempts, 5));
      logger.info(`[${this.opts.market} conn ${connIndex}] reconnecting in ${delay}ms (attempt ${reconnectAttempts})`);
      setTimeout(connect, delay);
    };

    connect();

    this.closers.push(() => {
      if (subscribeTimer) {
        clearTimeout(subscribeTimer);
        subscribeTimer = null;
      }
      clearInterval(restartTimer);
      ws?.close();
    });
  }

  private handleKline(data: RawKlineMessage): void {
    const msg = data;
    if (!msg || msg.e !== 'kline' || !msg.k) return;

    const candle: OneMinuteCandle = {
      symbol: `${msg.s}${this.opts.symbolSuffix}`,
      openTime: msg.k.t,
      closeTime: msg.k.T,
      open: msg.k.o,
      high: msg.k.h,
      low: msg.k.l,
      close: msg.k.c,
      volume: msg.k.v,
      isClosed: msg.k.x,
    };

    this.opts.onCandle(candle);
  }
}
