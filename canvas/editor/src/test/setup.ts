import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";

// globals is false, so Testing Library cannot register its own afterEach: unmount here, as the
// desktop's src/test/setup.ts does. Tasks 24-26 add the jsdom stubs their components need.
afterEach(() => {
  cleanup();
});
