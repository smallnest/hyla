declare namespace NodeJS {
  interface ProcessEnv {
    [key: string]: string | undefined;
  }
}

interface Buffer extends Uint8Array {
  toString(encoding?: string): string;
}

declare var process: {
  env: Record<string, string | undefined>;
  stdin: import("node:stream").Readable;
  stdout: import("node:stream").Writable;
  stderr: import("node:stream").Writable;
};

declare var Buffer: {
  byteLength(input: string, encoding?: string): number;
  isBuffer(obj: unknown): obj is Buffer;
};

declare module "node:stream" {
  export class EventEmitter {
    on(event: string, listener: (...args: any[]) => void): this;
    once(event: string, listener: (...args: any[]) => void): this;
    off(event: string, listener: (...args: any[]) => void): this;
    emit(event: string, ...args: any[]): boolean;
  }
  export class Readable extends EventEmitter {
    setEncoding(encoding: string): this;
  }
  export class Writable extends EventEmitter {
    write(chunk: string | Uint8Array): boolean;
    end(chunk?: string | Uint8Array): this;
  }
  export class Duplex extends Readable {
    write(chunk: string | Uint8Array): boolean;
    end(chunk?: string | Uint8Array): this;
  }
  export class PassThrough extends Duplex {}
}

declare module "node:net" {
  import type { Duplex } from "node:stream";
  export class Socket extends Duplex {}
  type NetSocket = Socket;
  export interface Server {
    listen(path: string, listeningListener?: () => void): this;
    close(callback?: (err?: Error) => void): this;
  }
  export function connect(path: string, connectionListener?: () => void): Socket;
  export function createServer(connectionListener?: (socket: Socket) => void): Server;
  namespace net {
    export type Socket = NetSocket;
  }
  const net: {
    connect: typeof connect;
    createServer: typeof createServer;
    Socket: typeof Socket;
  };
  export default net;
}

declare module "node:os" {
  export function tmpdir(): string;
  const os: { tmpdir: typeof tmpdir };
  export default os;
}

declare module "node:path" {
  export function join(...paths: string[]): string;
  const path: { join: typeof join };
  export default path;
}

declare module "node:fs" {
  export function mkdtempSync(prefix: string): string;
  export function rmSync(path: string, options?: { recursive?: boolean; force?: boolean }): void;
  const fs: { mkdtempSync: typeof mkdtempSync; rmSync: typeof rmSync };
  export default fs;
}

declare module "bun:test" {
  export function describe(name: string, fn: () => void): void;
  export function test(name: string, fn: () => void | Promise<void>): void;
  export interface ExpectMatchers<T = unknown> {
    toBe(expected: T): void;
    toEqual(expected: unknown): void;
    toContain(expected: string): void;
    readonly rejects: {
      toMatchObject(expected: Record<string, unknown>): Promise<void>;
    };
  }
  export function expect<T = unknown>(actual: T): ExpectMatchers<T>;
}
