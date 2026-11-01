import v8 from "node:v8";
import { afterAll } from "vitest";

// En Node.js, los worker threads de Vitest (tinypool) terminan con worker.terminate()
// y no vuelcan NODE_V8_COVERAGE a disco salvo que se invoque v8.takeCoverage() antes
// de cerrar cada worker.
afterAll(() => {
  if (process.env.NODE_V8_COVERAGE) {
    v8.takeCoverage();
  }
});
