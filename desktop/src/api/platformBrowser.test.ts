import { expect, it } from "vitest";
import { isGatewayHostname, isolatedBrowserURL } from "./platform";

const hostname = `${"a".repeat(32)}.${"b".repeat(32)}.localhost`;

it("accepts the full capability as two DNS-valid labels", () => {
  expect(isGatewayHostname(hostname)).toBe(true);
  expect(hostname.split(".").every(label => label.length <= 63)).toBe(true);
  expect(isolatedBrowserURL(`http://${hostname}:8731/`)).toBe(`http://${hostname}:8731/`);
});

it.each([
  `${"a".repeat(64)}.localhost`,
  `${"a".repeat(31)}.${"b".repeat(33)}.localhost`,
  `${"a".repeat(33)}.${"b".repeat(31)}.localhost`,
  `${"g".repeat(32)}.${"b".repeat(32)}.localhost`,
  `extra.${hostname}`,
  `${hostname}.example`,
])("rejects malformed or different gateway hostnames: %s", invalid => {
  expect(isGatewayHostname(invalid)).toBe(false);
  expect(isolatedBrowserURL(`http://${invalid}:8731/`)).toBeUndefined();
});

it("preserves credential, transport and control-origin exclusions", () => {
  expect(isolatedBrowserURL(`http://user:password@${hostname}:8731/`)).toBeUndefined();
  expect(isolatedBrowserURL(`https://${hostname}:8731/`)).toBeUndefined();
  expect(isolatedBrowserURL(window.location.origin)).toBeUndefined();
});

it("accepts the DNS-valid HTTPS isolated origin but rejects altered transports and padding", () => {
  const publicHost = `p-${"b".repeat(51)}a.previews.example.test`;
  expect(isolatedBrowserURL(`https://${publicHost}/app`)).toBe(`https://${publicHost}/app`);
  expect(isolatedBrowserURL(`http://${publicHost}/app`)).toBeUndefined();
  expect(isolatedBrowserURL(`https://${publicHost}:8443/app`)).toBeUndefined();
  expect(isolatedBrowserURL(`https://${publicHost.replace('a.previews', 'b.previews')}/app`)).toBeUndefined();
  expect(isolatedBrowserURL(`https://${publicHost.replace('example.test', 'localhost')}/app`)).toBeUndefined();
});
