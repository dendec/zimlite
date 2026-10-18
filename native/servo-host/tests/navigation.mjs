import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import vm from 'node:vm';

// Execute the viewer's actual trusted scripts without building the browser.
const source = readFileSync(new URL('../src/session.rs', import.meta.url), 'utf8');
const script = name => source.match(new RegExp(`const ${name}: &str = r#"([\\s\\S]*?)"#;`))[1];
const selectCommand = source.match(/"select" => Some\(format!\(\s*r#"([\s\S]*?)"#/)[1]
  .replaceAll('{{', '{').replaceAll('}}', '}')
  .replace('{LINK_STATE_JS}', script('LINK_STATE_JS'))
  .replace('{LINK_SELECTION_JS}', script('LINK_SELECTION_JS'));

function page(positions, selected = -1, scroll = 1000, hidden = []) {
  const state = { scroll, selected };
  const links = positions.map((top, i) => ({
    parentElement: null, style: {},
    getClientRects: () => [{ width: 100, height: 20, top: top - state.scroll,
      bottom: top - state.scroll + 20, left: 10, right: 110 }],
    setAttribute: () => { state.selected = i; },
    removeAttribute: () => { if (state.selected === i) state.selected = -1; },
    focus: () => {},
    scrollIntoView: () => { state.scroll = top; },
  }));
  const context = vm.createContext({
    innerHeight: 400, innerWidth: 640,
    getComputedStyle: a => ({ visibility: hidden.includes(links.indexOf(a)) ? 'hidden' : 'visible' }),
    document: {
      querySelectorAll: selector => selector === 'a[href]' ? links : state.selected < 0 ? [] : [links[state.selected]],
      querySelector: () => links[state.selected] ?? null,
    },
  });
  const run = code => vm.runInContext(`(() => {${script('LINK_STATE_JS')}${script('LINK_SELECTION_JS')}${code}})()`, context);
  return { state, move: direction => run(`zimliteMoveLink(${direction});`),
    restore: index => vm.runInContext(selectCommand.replace('{}', String(index)), context) };
}

test('restoring an offscreen selection preserves scroll', () => {
  const p = page([100, 2000]);
  p.restore(0);
  assert.equal(p.state.selected, 0);
  assert.equal(p.state.scroll, 1000);
});
test('restoring no selection preserves scroll and clears selection', () => {
  const p = page([100], 0);
  p.restore(-1);
  assert.equal(p.state.selected, -1);
  assert.equal(p.state.scroll, 1000);
});
for (const direction of [-1, 1]) {
  test(`empty viewport: choose nearest link in direction ${direction}`, () => {
    const p = page([100, 900, 1500, 2000], 0);
    p.move(direction);
    assert.equal(p.state.selected, direction > 0 ? 2 : 1);
    assert.equal(p.state.scroll, direction > 0 ? 1500 : 900);
  });
  test(`reenter at first visible link without scrolling, direction ${direction}`, () => {
    const p = page([100, 1100, 1200, 2000], 0);
    p.move(direction);
    assert.equal(p.state.selected, 1);
    assert.equal(p.state.scroll, 1000);
  });
}
test('skip hidden offscreen links', () => {
  const p = page([100, 1500, 2000], -1, 1000, [1]);
  p.move(1);
  assert.equal(p.state.selected, 2);
});
test('continue from current visible selection', () => {
  const p = page([1100, 1200, 2000], 0);
  p.move(1);
  assert.equal(p.state.selected, 1);
  assert.equal(p.state.scroll, 1000);
});
test('empty article is safe', () => {
  const p = page([]);
  p.move(1);
  assert.equal(p.state.selected, -1);
});
