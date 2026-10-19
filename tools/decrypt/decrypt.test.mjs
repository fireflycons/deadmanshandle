// Checks decrypt.html's decryption against the Go test vector, so that the
// page and internal/doccrypt agree on the format. Run with Node 20 or later
// (WebCrypto as browsers have it): node --test tools/decrypt/decrypt.test.mjs
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const here = new URL(".", import.meta.url);
const testdata = new URL("../../internal/doccrypt/testdata/", here);

// The page's decryption code, between its BEGIN and END markers
const html = readFileSync(new URL("decrypt.html", here), "utf8");
const code = html.match(/\/\/ BEGIN decrypt\r?\n([\s\S]*?)\/\/ END decrypt/)[1];
const { decryptDocument } = new Function(code + "\nreturn { decryptDocument, OpenError };")();

const vector = JSON.parse(readFileSync(new URL("vector.json", testdata), "utf8"));
const encrypted = new Uint8Array(readFileSync(new URL("vector.dmh", testdata)));

async function failure(bytes, password) {
  try {
    await decryptDocument(bytes, password);
  } catch (err) {
    return err.code;
  }
  return "opened";
}

test("opens the Go test vector", async () => {
  const { name, content } = await decryptDocument(encrypted, vector.password);
  assert.equal(name, vector.name);
  assert.equal(new TextDecoder().decode(content), vector.content);
});

test("refuses a wrong password", async () => {
  assert.equal(await failure(encrypted, "not it"), "wrong-password");
  assert.equal(await failure(encrypted, vector.password.toUpperCase()), "wrong-password");
});

test("refuses a changed or damaged file", async () => {
  for (const at of [10, 30, encrypted.length - 20, encrypted.length - 1]) {
    const changed = encrypted.slice();
    changed[at] ^= 1;
    assert.equal(await failure(changed, vector.password), "wrong-password", `byte ${at}`);
  }
  assert.equal(await failure(encrypted.slice(0, encrypted.length - 1), vector.password), "wrong-password");
  assert.equal(await failure(encrypted.slice(0, 10), vector.password), "wrong-password");
  const absurd = encrypted.slice();
  absurd.set([0xff, 0xff, 0xff, 0xff], 5);
  assert.equal(await failure(absurd, vector.password), "wrong-password");
});

test("recognises other files and later versions", async () => {
  assert.equal(await failure(new TextEncoder().encode("%PDF-1.7"), vector.password), "not-ours");
  const later = encrypted.slice();
  later[4] = 2;
  assert.equal(await failure(later, vector.password), "version");
});
