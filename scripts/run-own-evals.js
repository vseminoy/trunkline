#!/usr/bin/env node
/**
 * run-own-evals.js — Tier-2 (trigger & routing) для СОБСТВЕННЫХ скиллов репозитория.
 *
 * Зачем отдельный раннер: `agent-skills/scripts/run-evals.js` токенизирует
 * `[^a-z0-9\s-]` → пробел, то есть кириллица выбрасывается целиком и все
 * документы получаются пустыми. Описания наших скиллов — на русском, поэтому
 * здесь тот же алгоритм (TF-IDF по name×2 + description, косинус), но с
 * Unicode-токенизацией и грубым стеммером для русского.
 *
 * Проверяет:
 *   1. Coverage      — у каждого скилла есть файл кейсов.
 *   2. Positive      — целевой скилл попадает в top_k по своему промпту.
 *   3. Negative      — по чужому промпту целевой скилл top-1 НЕ занимает.
 *   4. Collision     — никакая пара описаний не похожа выше порога.
 *
 * Tier 2 — лексическое приближение маршрутизации, не семантика. Провал обычно
 * означает «исправь description», а не «исправь эval».
 *
 * Запуск: node scripts/run-own-evals.js
 * Коды выхода: 0 — чисто, 1 — есть ошибки.
 */

'use strict';

const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..');
const CASES_DIR = path.join(ROOT, 'evals', 'cases');

// Корни, где живут наши скиллы.
const SKILL_ROOTS = [
  path.join(ROOT, 'core', 'analytic', 'skills'),
  path.join(ROOT, 'core', 'skills'),
];

const COLLISION_THRESHOLD = 0.6;
const MIN_RANK1_RATIO = 0.7;

// ─── Текстовый конвейер (Unicode) ────────────────────────────────────────────

const STOP = new Set([
  // ru
  'и', 'в', 'во', 'не', 'что', 'он', 'на', 'я', 'с', 'со', 'как', 'а', 'то',
  'все', 'она', 'так', 'его', 'но', 'да', 'ты', 'к', 'у', 'же', 'вы', 'за',
  'бы', 'по', 'только', 'ее', 'мне', 'было', 'вот', 'от', 'меня', 'еще',
  'нет', 'о', 'из', 'ему', 'теперь', 'когда', 'даже', 'ну', 'вдруг', 'ли',
  'если', 'уже', 'или', 'ни', 'быть', 'был', 'него', 'до', 'вас', 'нибудь',
  'опять', 'уж', 'вам', 'ведь', 'там', 'потом', 'себя', 'ничего', 'ей',
  'может', 'они', 'тут', 'где', 'есть', 'надо', 'ней', 'для', 'мы', 'тебя',
  'их', 'чем', 'была', 'сам', 'чтоб', 'без', 'будто', 'чего', 'раз', 'тоже',
  'себе', 'под', 'будет', 'ж', 'тогда', 'кто', 'этот', 'того', 'потому',
  'этого', 'какой', 'совсем', 'ним', 'здесь', 'этом', 'один', 'почти', 'мой',
  'над', 'после', 'над', 'больше', 'тот', 'через', 'эти', 'нас', 'про',
  'всего', 'них', 'какая', 'много', 'разве', 'три', 'эту', 'моя', 'впрочем',
  'свою', 'этой', 'перед', 'иногда', 'лучше', 'чуть', 'том', 'нельзя',
  'такой', 'им', 'более', 'всю', 'между', 'надо',
  // en
  'a', 'an', 'and', 'are', 'as', 'at', 'be', 'by', 'for', 'from', 'in', 'is',
  'it', 'of', 'on', 'or', 'the', 'this', 'to', 'use', 'when', 'with',
]);

// Грубый стеммер: отрезает частотные русские окончания, затем усекает
// до 6 символов. Не морфология — достаточно, чтобы «подзадача»/«подзадачи»/
// «подзадачам» схлопнулись в один терм.
const RU_SUFFIXES = [
  'ированиями', 'ированием', 'ированию', 'ирования', 'ировании',
  'ями', 'ами', 'иями', 'ов', 'ев', 'ей', 'ий', 'ый', 'ой', 'ая', 'яя',
  'ое', 'ее', 'ые', 'ие', 'ых', 'их', 'ую', 'юю', 'ам', 'ям', 'ах', 'ях',
  'ом', 'ем', 'ла', 'ло', 'ли', 'ть', 'ся', 'ет', 'ут', 'ют', 'ит', 'ят',
  'а', 'я', 'о', 'е', 'у', 'ю', 'ы', 'и', 'ь',
];

function stem(t) {
  if (/[а-яё]/.test(t)) {
    for (const suf of RU_SUFFIXES) {
      if (t.length > suf.length + 3 && t.endsWith(suf)) {
        t = t.slice(0, -suf.length);
        break;
      }
    }
    return t.slice(0, 6);
  }
  // латиница — как в апстримном раннере, упрощённо
  for (const suf of ['ing', 'ed', 'es', 'al']) {
    if (t.length > suf.length + 3 && t.endsWith(suf)) {
      t = t.slice(0, -suf.length);
      break;
    }
  }
  if (t.length > 3 && t.endsWith('s') && !t.endsWith('ss')) t = t.slice(0, -1);
  return t;
}

function tokenize(text) {
  return text
    .toLowerCase()
    .replace(/ё/g, 'е')
    .replace(/[^\p{L}\p{N}\s-]/gu, ' ')
    .split(/[\s-]+/)
    .filter((t) => t.length > 2 && !STOP.has(t))
    .map(stem);
}

function termFreq(tokens) {
  const tf = new Map();
  for (const t of tokens) tf.set(t, (tf.get(t) || 0) + 1);
  return tf;
}

function buildCorpus(skills) {
  const docs = new Map();
  for (const s of skills) {
    const nameTokens = tokenize(s.name.replace(/-/g, ' '));
    const tokens = [...nameTokens, ...nameTokens, ...tokenize(s.description)];
    docs.set(s.name, termFreq(tokens));
  }
  const df = new Map();
  for (const tf of docs.values()) {
    for (const term of tf.keys()) df.set(term, (df.get(term) || 0) + 1);
  }
  const n = docs.size;
  const idf = (term) => Math.log(1 + n / (1 + (df.get(term) || 0)));
  return { docs, idf };
}

function vec(tf, idf) {
  const v = new Map();
  for (const [term, f] of tf) v.set(term, f * idf(term));
  return v;
}

function cosine(a, b) {
  let dot = 0, na = 0, nb = 0;
  for (const [t, w] of a) {
    na += w * w;
    const bw = b.get(t);
    if (bw) dot += w * bw;
  }
  for (const w of b.values()) nb += w * w;
  if (!na || !nb) return 0;
  return dot / (Math.sqrt(na) * Math.sqrt(nb));
}

function rankSkills(prompt, corpus) {
  const pv = vec(termFreq(tokenize(prompt)), corpus.idf);
  const scores = [];
  for (const [name, tf] of corpus.docs) {
    scores.push({ name, score: cosine(pv, vec(tf, corpus.idf)) });
  }
  scores.sort((a, b) => b.score - a.score);
  return scores;
}

// ─── Загрузка ────────────────────────────────────────────────────────────────

function loadSkills(roots = SKILL_ROOTS, own = true) {
  const skills = [];
  for (const root of roots) {
    if (!fs.existsSync(root)) continue;
    for (const dir of fs.readdirSync(root)) {
      const file = path.join(root, dir, 'SKILL.md');
      if (!fs.existsSync(file)) continue;
      const src = fs.readFileSync(file, 'utf8');
      const m = src.match(/^---[ \t]*\r?\n([\s\S]*?)\r?\n---[ \t]*(?:\r?\n|$)/);
      if (!m) continue;
      const name = (m[1].match(/^name:\s*(.+)$/m) || [])[1];
      const description = (m[1].match(/^description:\s*(.+)$/m) || [])[1];
      if (name && description) {
        skills.push({ name: name.trim(), description: description.trim(), dir: path.join(root, dir), own });
      }
    }
  }
  return skills;
}

function loadCases() {
  if (!fs.existsSync(CASES_DIR)) return [];
  return fs.readdirSync(CASES_DIR)
    .filter((f) => f.endsWith('.json'))
    .map((f) => {
      const raw = fs.readFileSync(path.join(CASES_DIR, f), 'utf8');
      try {
        return { file: f, data: JSON.parse(raw) };
      } catch (e) {
        return { file: f, parseError: e.message };
      }
    });
}

// ─── Прогон ──────────────────────────────────────────────────────────────────

function main() {
  const own = loadSkills();
  // Каталог agent-skills подмешивается в корпус как конкуренты: в реальной
  // сессии агент выбирает из всех скиллов сразу. Кейсы для них не требуются.
  const upstream = loadSkills([path.join(ROOT, 'agent-skills', 'skills')], false);
  const skills = [...own, ...upstream];
  const cases = loadCases();
  const corpus = buildCorpus(skills);
  const skillNames = new Set(skills.map((s) => s.name));

  let errors = 0, warnings = 0, positives = 0, rank1 = 0, passed = 0;

  console.log(`Tier 2: ${own.length} собственных скиллов (+${upstream.length} из agent-skills в корпусе), ${cases.length} файлов кейсов\n`);

  for (const s of own) {
    if (!cases.some((c) => c.file === `${s.name}.json`)) {
      console.log(`  ✗  ${s.name}: нет файла кейсов (evals/cases/${s.name}.json)`);
      errors++;
    }
  }

  for (const c of cases) {
    if (c.parseError) {
      console.log(`  ✗  ${c.file}: битый JSON — ${c.parseError}`);
      errors++;
      continue;
    }
    const expected = c.file.replace(/\.json$/, '');
    if (c.data.skill_name !== expected) {
      console.log(`  ✗  ${c.file}: skill_name "${c.data.skill_name}" не совпадает с именем файла`);
      errors++;
    }
    if (!skillNames.has(expected)) {
      console.log(`  ✗  ${c.file}: такого скилла нет`);
      errors++;
      continue;
    }

    for (const t of c.data.trigger?.positive || []) {
      positives++;
      const topK = t.top_k || 3;
      const ranking = rankSkills(t.prompt, corpus);
      const idx = ranking.findIndex((r) => r.name === expected);
      const hit = ranking[idx];
      if (idx === 0 && hit.score > 0) rank1++;
      if (idx >= 0 && idx < topK && hit.score > 0) {
        passed++;
      } else {
        const top = ranking.slice(0, 3).map((r) => `${r.name}(${r.score.toFixed(2)})`).join(', ');
        console.log(`  ✗  ${expected}: промпт не попал в top-${topK} — "${t.prompt}"`);
        console.log(`       топ: ${top}`);
        errors++;
      }
    }

    for (const t of c.data.trigger?.negative || []) {
      const ranking = rankSkills(t.prompt, corpus);
      if (ranking[0].name === expected && ranking[0].score > 0) {
        console.log(`  ✗  ${expected}: занял top-1 на чужом промпте — "${t.prompt}"`);
        errors++;
      } else if (t.owner && ranking[0].name !== t.owner && skillNames.has(t.owner)) {
        console.log(`  ⚠  ожидался владелец "${t.owner}", top-1 — "${ranking[0].name}" ("${t.prompt}")`);
        warnings++;
      }
    }
  }

  // Коллизии описаний
  console.log('');
  // Интересуют только пары, где хотя бы один скилл наш: коллизии внутри
  // апстримного каталога — забота его собственных evals.
  const ownNames = new Set(own.map((s) => s.name));
  const names = [...corpus.docs.keys()];
  for (let i = 0; i < names.length; i++) {
    for (let j = i + 1; j < names.length; j++) {
      if (!ownNames.has(names[i]) && !ownNames.has(names[j])) continue;
      const sim = cosine(vec(corpus.docs.get(names[i]), corpus.idf), vec(corpus.docs.get(names[j]), corpus.idf));
      if (sim >= COLLISION_THRESHOLD) {
        console.log(`  ✗  коллизия описаний: ${names[i]} ↔ ${names[j]} (cos=${sim.toFixed(2)})`);
        errors++;
      }
    }
  }

  const ratio = positives ? rank1 / positives : 1;
  console.log(`\npositive: ${passed}/${positives} в top-k, rank-1 ${rank1}/${positives} (${(ratio * 100).toFixed(0)}%)`);
  if (positives && ratio < MIN_RANK1_RATIO) {
    console.log(`  ⚠  доля rank-1 ниже порога ${(MIN_RANK1_RATIO * 100).toFixed(0)}%`);
    warnings++;
  }
  const status = errors ? 'FAILED' : warnings ? 'PASSED WITH WARNINGS' : 'PASSED';
  console.log(`${errors} error(s), ${warnings} warning(s) — ${status}`);
  if (errors) process.exit(1);
}

main();
