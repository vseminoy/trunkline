#!/usr/bin/env node
/**
 * lint-own-skills.js — Tier-1 (структура) для СОБСТВЕННЫХ скиллов репозитория.
 *
 * `agent-skills/scripts/validate-skills.js` жёстко смотрит только в свой
 * `skills/`, поэтому здесь тот же линтер (`scripts/lib/skill-lint.js`)
 * применяется к нашим каталогам скиллов.
 *
 * Ядро с Ф2.3 на английском, поэтому апстримный линтер применяется как есть,
 * без поправок на язык: описание обязано содержать "Use when …".
 *
 * Запуск: node scripts/lint-own-skills.js
 */
'use strict';
const fs = require('fs'), path = require('path');
const ROOT = path.resolve(__dirname, '..');
const { lintSkill } = require(path.join(ROOT, 'agent-skills/scripts/lib/skill-lint.js'));

const ROOTS = [
  path.join(ROOT, 'core/analytic/skills'),
  path.join(ROOT, 'core/skills'),
];

// Общий словарь известных скиллов: свои + все из agent-skills,
// чтобы кросс-ссылки на `planning-and-task-breakdown` и т.п. не считались мёртвыми.
const known = new Set(fs.readdirSync(path.join(ROOT, 'agent-skills/skills')));
const entries = [];
for (const dir of ROOTS) {
  for (const d of fs.readdirSync(dir)) {
    if (!fs.statSync(path.join(dir, d)).isDirectory()) continue;
    known.add(d);
    entries.push([dir, d]);
  }
}

// Подавления триггер-проверки больше нет: с Ф2.3 ядро на английском, и
// описания обязаны нести каноническое "Use when …" — ту же формулировку,
// на которую рассчитан апстримный линтер.
let E = 0, W = 0;
for (const [dir, name] of entries) {
  const { errors, warnings } = lintSkill(name, dir, known);
  E += errors.length; W += warnings.length;
  const rel = path.relative(ROOT, path.join(dir, name));
  if (!errors.length && !warnings.length) { console.log(`  ✓  ${rel}`); continue; }
  console.log(`${errors.length ? '  ✗ ' : '  ⚠ '} ${rel}`);
  for (const m of errors)   console.log(`       ERROR: ${m}`);
  for (const m of warnings) console.log(`       WARN:  ${m}`);
}
console.log(`\n${entries.length} skills checked — ${E} error(s), ${W} warning(s)`);
if (E) process.exit(1);
