import { describe, expect, it } from "vitest";

import { formatElapsed, formatWaiting } from "./time";

const MIN = 60_000;
const HOUR = 60 * MIN;
const DAY = 24 * HOUR;

const DASH = "—"; // em dash: what an unparseable timestamp renders as

const BASE = "2026-09-18T09:00:00.000Z";
const now = Date.parse(BASE);
const ago = (ms: number): string => new Date(now - ms).toISOString();

describe("formatElapsed", () => {
  it("floors to whole minutes below an hour, so it never shows time that has not elapsed", () => {
    expect(formatElapsed(ago(0), now)).toBe("0m");
    expect(formatElapsed(ago(59_000), now)).toBe("0m");
    expect(formatElapsed(ago(41 * MIN), now)).toBe("41m");
    expect(formatElapsed(ago(41 * MIN + 59_000), now)).toBe("41m");
    expect(formatElapsed(ago(59 * MIN), now)).toBe("59m");
  });

  it("zero-pads the minutes once there is an hour, and leaves the hour unpadded", () => {
    expect(formatElapsed(ago(60 * MIN), now)).toBe("1h 00m");
    expect(formatElapsed(ago(HOUR + 6 * MIN), now)).toBe("1h 06m");
    expect(formatElapsed(ago(23 * HOUR + 59 * MIN), now)).toBe("23h 59m");
  });

  it("switches to days at 24 hours and keeps the hour when it is not zero", () => {
    expect(formatElapsed(ago(DAY), now)).toBe("1d");
    expect(formatElapsed(ago(2 * DAY), now)).toBe("2d");
    expect(formatElapsed(ago(DAY + 3 * HOUR + 40 * MIN), now)).toBe("1d 03h");
    expect(formatElapsed(ago(9 * DAY + 23 * HOUR), now)).toBe("9d 23h");
  });

  it("clamps clock skew to zero rather than printing a negative duration", () => {
    expect(formatElapsed(new Date(now + 5_000).toISOString(), now)).toBe("0m");
  });

  it("renders an em dash for a timestamp it cannot parse", () => {
    expect(formatElapsed("", now)).toBe(DASH);
    expect(formatElapsed("not a date", now)).toBe(DASH);
  });

  it("renders an em dash when nowMs is not a finite number", () => {
    expect(formatElapsed(BASE, Number.NaN)).toBe(DASH);
    expect(formatElapsed(BASE, Number.POSITIVE_INFINITY)).toBe(DASH);
  });

  it("is pure: it never reads the wall clock, so one render tick prints one number", () => {
    const realNow = Date.now;
    Date.now = () => {
      throw new Error("formatElapsed must not read the wall clock");
    };
    try {
      expect(formatElapsed(ago(41 * MIN), now)).toBe("41m");
    } finally {
      Date.now = realNow;
    }
  });
});

describe("formatWaiting", () => {
  it("prefixes the elapsed form — contract §7.4's worked example", () => {
    expect(formatWaiting(ago(5 * HOUR + 51 * MIN), now)).toBe("waiting 5h 51m");
  });

  it("says waiting 0m rather than nothing at all in the first minute", () => {
    expect(formatWaiting(BASE, now)).toBe("waiting 0m");
  });

  it("does not prefix a value it could not compute", () => {
    expect(formatWaiting("", now)).toBe(DASH);
  });
});
