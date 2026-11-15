import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import vm from "node:vm";

const files = new Map([
  [new URL("../dist/v1/wloc.js", import.meta.url), "c6de58acd18538589657de890d92a1b9a2cea721806801d3c807075b089719f1"],
  [new URL("../dist/v1/wloc-settings.js", import.meta.url), "6ada4c9a44ac5321073fe923ff89ecd7f212c1290126a368cc1c5d9723e6cc8e"],
  [new URL("../LICENSE", import.meta.url), "8486a10c4393cee1c25392769ddd3b2d6c242d6ec7928e1414efff7dfb2f07ef"]
]);

test("vendored upstream files retain their pinned SHA-256 hashes", async () => {
  for (const [file, expectedHash] of files) {
    const contents = await readFile(file);
    const actualHash = createHash("sha256").update(contents).digest("hex");
    assert.equal(actualHash, expectedHash, file.pathname);
  }
});

test("settings bundle saves and queries wloc_settings in Shadowrocket", async () => {
  const source = await readFile(new URL("../dist/v1/wloc-settings.js", import.meta.url), "utf8");
  const storage = new Map();

  const run = (url) => {
    let result;
    vm.runInNewContext(source, {
      $rocket: {},
      $request: { url },
      $persistentStore: {
        read: (key) => storage.get(key) ?? null,
        write: (value, key) => {
          storage.set(key, value);
          return true;
        }
      },
      $done: (value) => { result = value; },
      console
    });
    return JSON.parse(result.response.body);
  };

  const saved = run("https://gs-loc.apple.com/wloc-settings/save?lon=121.1&lat=31.2&acc=25");
  assert.equal(saved.success, true);
  assert.equal(storage.has("wloc_settings"), true);

  const queried = run("https://gs-loc.apple.com/wloc-settings/save?action=query");
  assert.deepEqual(
    { success: queried.success, longitude: queried.longitude, latitude: queried.latitude, accuracy: queried.accuracy },
    { success: true, longitude: 121.1, latitude: 31.2, accuracy: 25 }
  );

  const zeroSaved = run("https://gs-loc.apple.com/wloc-settings/save?lon=0&lat=0&acc=25");
  assert.equal(zeroSaved.success, true);
  const zeroQueried = run("https://gs-loc.apple.com/wloc-settings/save?action=query");
  assert.deepEqual(
    { success: zeroQueried.success, longitude: zeroQueried.longitude, latitude: zeroQueried.latitude },
    { success: true, longitude: 0, latitude: 0 }
  );
});

test("WLOC runtime accepts zero-valued saved and argument coordinates", async () => {
  const source = await readFile(new URL("../dist/v1/wloc.js", import.meta.url), "utf8");
  assert.match(source, /null!=a\.longitude/);
  assert.match(source, /null!=a\.latitude/);
  assert.match(source, /void 0!==e\.longitude/);
  assert.match(source, /void 0!==e\.latitude/);
  assert.doesNotMatch(source, /a\.longitude&&\(r\.longitude=/);
  assert.doesNotMatch(source, /a\.latitude&&\(r\.latitude=/);
});
