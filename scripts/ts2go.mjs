#!/usr/bin/env node
/**
 * Generator: ports @harizinside/doku's TS product modules to Go.
 *
 * Reads ../doku/src/products/*.ts via the TypeScript compiler API
 * (typescript is installed in ../doku/node_modules — see npm install there)
 * and writes zz_<module>_gen.go for each of the 8 "uniform" modules:
 * o2o, directDebitLegacy, directDebitSnap, paylater, directTransfer,
 * kki, subAccountV1, subAccountV2.
 *
 * Hand-written modules (virtualAccount*, creditCard, checkout, emoney,
 * misc, settlement) are NOT touched here — they live in their own .go
 * files. This script only emits types for the uniform modules and the
 * methods whose bodies are pure `client.requestNonSnap(requestSnap)` calls.
 *
 * Usage:
 *   node scripts/ts2go.mjs ../doku
 *   gofmt -w .
 */
import { readFileSync, writeFileSync, existsSync, readdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const pkgRoot = resolve(here, "..");
const args = process.argv.slice(2);
const dokuRoot = args[0] ?? resolve(pkgRoot, "..", "doku");

// Resolve typescript from the doku package's node_modules so the runtime
// doesn't need a separate install. Falls back to the global install.
function loadTs() {
  const candidates = [
    join(dokuRoot, "node_modules", "typescript", "lib", "typescript.js"),
    join(pkgRoot, "node_modules", "typescript", "lib", "typescript.js"),
  ];
  for (const p of candidates) {
    if (existsSync(p)) {
      return import(pathToFileURL(p));
    }
  }
  throw new Error("typescript not found — run `npm install typescript` in ../doku");
}
import { pathToFileURL } from "node:url";
const ts = await loadTs();

const UNIFORM = new Set([
  "o2o.ts",
  "directDebitLegacy.ts",
  "directDebitSnap.ts",
  "paylater.ts",
  "directTransfer.ts",
  "kki.ts",
  "subAccountV1.ts",
  "subAccountV2.ts",
]);

// Bank prefixes — every fn in a uniform module is bank-prefixed; the prefix
// tells us where to slot the function (and to name the service struct).
const BANK_PREFIXES = [
  "ovo", "cimb", "allo", "bri", "mandiri", "dana", "shopeepay",
];

const fileMap = {};
const rootNames = [];
const fModule = new WeakMap();
function currentGoName(f) { return fModule.get(f); }
for (const name of readdirSync(join(dokuRoot, "src", "products"))) {
  if (!UNIFORM.has(name)) continue;
  const abs = join(dokuRoot, "src", "products", name);
  fileMap[name] = readFileSync(abs, "utf8");
  rootNames.push(abs);
}

const program = ts.createProgram({
  rootNames,
  options: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, strict: true, allowJs: false },
});

const checker = program.getTypeChecker();

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const pascal = (s) => (s ? s[0].toUpperCase() + s.slice(1) : s);
const stripQuotes = (s) => s.replace(/^["'\x60]|["'\x60]$/g, "");

// Initialisms we preserve PascalCase → ALL-CAPS for in Go field names.
const INITIALISMS = new Set([
  "id", "ids", "url", "urls", "va", "api", "otp", "emoney", "ovo",
  "doku", "sbs", "sac", "bnc", "btn", "bri", "bni", "bsi", "cimb",
  "dana", "shopeepay", "allo", "mandiri", "iso", "ttl", "tv", "ip",
  "ocr", "ml", "ui", "ux", "json", "xml", "yaml", "http", "https",
  "pdf", "csv", "tts", "asn", "uuid",
]);

/**
 * Convert a snake_case TS key to a Go-friendly PascalCase identifier.
 * `invoice_number` → `InvoiceNumber`, `virtualAccountNo` stays put,
 * `customer_no` → `CustomerNo`, `partnerServiceId` stays put, etc.
 */
function pascalKey(key) {
  // Split on underscores and camelCase boundaries.
  const parts = key.split(/[_\s]+/).filter(Boolean).flatMap((seg) =>
    seg.replace(/([a-z0-9])([A-Z])/g, "$1 $2").split(/\s+/),
  );
  // Capitalize first char of every segment (already capitalized for camelCase
  // segments like "Id" → "Id"). Then handle trailing initialisms: anything
  // ending in `Id`, `Url`, `Va`, `Vat`, `Emoney`, etc. becomes upper-case.
  let joined = parts.map(pascal).join("");
  // Sort initialisms by length desc so longer matches win (e.g. "Emoney" before "M").
  const sortedInit = [...INITIALISMS].sort((a, b) => b.length - a.length);
  for (const word of sortedInit) {
    const pascalWord = pascal(word);
    const upper = word.toUpperCase();
    if (joined === pascalWord) return upper;
    if (joined.endsWith(pascalWord) && joined.length > pascalWord.length) {
      joined = joined.slice(0, -pascalWord.length) + upper;
      break;
    }
  }
  return joined;
}

function commentText(node) {
  const text = node.getSourceFile().getText();
  // Use `node.pos` (the node's actual start) so we get the JSDoc that's
  // immediately before the node (between the trivia and the actual code).
  const cmts = ts.getLeadingCommentRanges(text, node.pos) || [];
  return cmts
    .map((c) => text.slice(c.pos, c.end))
    .join("\n")
    .replace(/^\/\*\*?/, "")
    .replace(/\*\/$/, "")
    .split("\n")
    .map((l) => l.replace(/^\s*\*\s?/, "").trim())
    .filter(Boolean)
    .join("\n");
}

function typeRefText(node) {
  return node.getText().trim();
}

// ---------------------------------------------------------------------------
// Type translation
// ---------------------------------------------------------------------------

/**
 * Convert a TypeScript type node into a Go type expression.
 * For union / intersection / Record / unknown types we drop down to `any`.
 * For literal unions (`"SALE" | "INSTALLMENT"`) we keep the string union
 * as `string` (the TS file declares them on the wire as plain strings).
 */
function tsTypeToGo(node, seen = new Set()) {
  const k = ts.SyntaxKind[node.kind];
  switch (k) {
    case "StringKeyword": return "string";
    case "NumberKeyword": return "float64";
    case "BooleanKeyword": return "*bool";
    case "AnyKeyword":
    case "UnknownKeyword":
    case "NullKeyword":
    case "UndefinedKeyword":
    case "NeverKeyword":
    case "ObjectKeyword":
      return "any";
    case "LiteralType": {
      const lit = node.literal;
      if (ts.isStringLiteral(lit)) return "string";
      if (ts.isNumericLiteral(lit)) return "float64";
      if (lit.kind === ts.SyntaxKind.TrueKeyword || lit.kind === ts.SyntaxKind.FalseKeyword) return "*bool";
      return "any";
    }
    case "UnionType": {
      const nonNull = node.types.filter((t) => !isNullOrUndefined(t));
      if (nonNull.length === 0) return "any";
      const isAllLiteral = nonNull.every((t) => t.kind === ts.SyntaxKind.LiteralType);
      if (isAllLiteral) return "string";
      const allSame = nonNull.every((t) => tsTypeToGo(t, seen) === tsTypeToGo(nonNull[0], seen));
      if (allSame) return tsTypeToGo(nonNull[0], seen);
      return "any";
    }
    case "IntersectionType": {
      const parts = [];
      for (const t of node.types) {
        const g = tsTypeToGo(t, seen);
        if (g !== "any") parts.push(g);
      }
      return parts.length ? parts.join(" & ") : "any";
    }
    case "ArrayType": {
      const inner = tsTypeToGo(node.elementType, seen);
      return `[]${inner}`;
    }
    case "TupleType":
      return "any";
    case "ParenthesizedType":
      return tsTypeToGo(node.type, seen);
    case "TypeReference": {
      const name = node.typeName.getText();
      if (name === "Array") {
        const arg = node.typeArguments?.[0];
        return arg ? `[]${tsTypeToGo(arg, seen)}` : "any";
      }
      if (name === "Record" || name === "Partial" || name === "Pick" || name === "Omit" || name === "Required") {
        return "any";
      }
      return pascalKey(name);
    }
    case "TypeLiteral":
      // Object literal outside a property (e.g. in a union) — no named type.
      return "map[string]any";
    case "IndexAccessType":
    case "MappedType":
    case "ConditionalType":
    case "TemplateLiteralType":
    default:
      return "any";
  }
}

function isNullOrUndefined(node) {
  return node.kind === ts.SyntaxKind.UndefinedKeyword
      || node.kind === ts.SyntaxKind.NullKeyword
      || (node.kind === ts.SyntaxKind.LiteralType && node.literal.kind === ts.SyntaxKind.NullKeyword);
}

// ---------------------------------------------------------------------------
// Walk and emit
// ---------------------------------------------------------------------------

const typeDecls = new Map(); // module → Map<Name, GoStruct>
const fnDecls   = new Map(); // module → FunctionEntry[]

for (const [fileName, source] of Object.entries(fileMap)) {
  const moduleKey = fileName.replace(/\.ts$/, "");
  typeDecls.set(moduleKey, new Map());
  fnDecls.set(moduleKey, []);

  const abs = join(dokuRoot, "src", "products", fileName);
  const sf = program.getSourceFile(abs);
  if (!sf) { console.error("! source not in program:", abs); continue; }
  for (const stmt of sf.statements) {
    if (ts.isInterfaceDeclaration(stmt)) {
      collectInterface(stmt, moduleKey);
    } else if (ts.isFunctionDeclaration(stmt) && stmt.name) {
      collectFunction(stmt, moduleKey);
    }
  }
}

function collectInterface(node, moduleKey) {
  const name = pascal(node.name.text);
  // Skip the synthetic *SnapOptions interface — we generate it ourselves.
  if (name.endsWith("SnapOptions")) return;
  // Index signatures (`[key: string]: unknown`) are dropped.
  registerStruct(moduleKey, name, node.members, commentText(node));
}

// Registers a struct type; inline object literals in its fields become their
// own named structs (`<Parent><Field>`) so callers can build them as literals.
function registerStruct(moduleKey, name, members, jsdoc) {
  const fields = members.filter(ts.isPropertySignature).map((m) => emitProperty(m, name, moduleKey));
  typeDecls.get(moduleKey).set(name, { name, fields, jsdoc, moduleKey });
}

function emitProperty(m, parent, moduleKey) {
  const rawName = m.name.getText().replace(/^["']|["']$/g, "");
  const goName = pascalKey(rawName);
  const isOpt = !!m.questionToken;
  const doc = commentText(m);
  const tag = `\`json:"${rawName}${isOpt ? ",omitempty" : ""}"\``;

  const isArr = ts.isArrayTypeNode(m.type);
  const el = isArr ? m.type.elementType : m.type;
  if (el.kind === ts.SyntaxKind.TypeLiteral) {
    const structName = parent + goName;
    registerStruct(moduleKey, structName, el.members, "");
    const type = isArr ? `[]${structName}` : isOpt ? `*${structName}` : structName;
    return { raw: rawName, name: goName, type, doc, isOpt, tag };
  }
  return { raw: rawName, name: goName, type: tsTypeToGo(m.type), doc, isOpt, tag };
}

function collectFunction(node, moduleKey) {
  let scheme = null;
  let path = null;
  let useSharedKey = false;
  let channelIdExpr = null;
  let customerTokenExpr = null;
  let params = [];
  let bodyArg = null;
  let httpMethod = "POST";

  for (const stmt of node.body.statements) {
    if (!ts.isReturnStatement(stmt) || !stmt.expression) continue;
    const expr = stmt.expression;
    if (!ts.isCallExpression(expr)) continue;
    const callee = expr.expression;
    if (!ts.isPropertyAccessExpression(callee) || !ts.isIdentifier(callee.expression) || callee.expression.text !== "client") continue;
    const method = callee.name.text;
    if (method !== "requestNonSnap" && method !== "requestSnap") continue;
    const args = expr.arguments;

    if (method === "requestNonSnap") {
      scheme = "nonSnap";
      path = stripQuotes(args[1]?.getText() ?? "");
      httpMethod = stripQuotes((args[0] && args[0].getText()) ?? "POST");
    } else {
      const optsLit = args[2];
      if (!ts.isObjectLiteralExpression(optsLit)) continue;
      for (const p of optsLit.properties) {
        if (ts.isPropertyAssignment(p)) {
          const k = p.name.getText();
          if (k === "useSharedKey" && p.initializer.kind === ts.SyntaxKind.TrueKeyword) useSharedKey = true;
          if (k === "channelId") channelIdExpr = p.initializer.getText();
          if (k === "customerToken") customerTokenExpr = p.initializer.getText();
        }
      }
      scheme = useSharedKey ? "snapShared" : "snap";
      path = stripQuotes(args[1]?.getText() ?? "");
      httpMethod = stripQuotes((args[0] && args[0].getText()) ?? "POST");
    }

    if (scheme === "nonSnap") {
      const b = args[2];
      if (b && (b.kind !== ts.SyntaxKind.Identifier || b.text !== "undefined")) {
        bodyArg = b.getText();
      }
    } else if (scheme === "snap" || scheme === "snapShared") {
      const optsLit = args[2];
      for (const p of optsLit?.properties ?? []) {
        if (ts.isPropertyAssignment(p) && p.name.getText() === "body") {
          bodyArg = p.initializer.getText();
          break;
        }
      }
    }
    break;
  }
  if (!scheme || path == null) return;

  const templ = path.match(/\$\{(\w+)\}/g);
  if (templ) {
    for (const t of templ) params.push(t.slice(2, -1));
    // Convert ${foo} → %s so the Go fmt.Sprintf call works.
    path = path.replace(/\$\{\w+\}/g, "%s");
  }

  const entry = {
    fnName: node.name.text,
    jsdoc: commentText(node),
    scheme,
    path,
    httpMethod,
    params,
    bodyArg,
    useSharedKey,
    channelIdExpr,
    customerTokenExpr,
  };
  fModule.set(entry, moduleKey);
  fnDecls.get(moduleKey).push(entry);
}

// Translate a TS expression like `options?.channelId` into Go.
// Recognised forms: `options?.foo`, `options?.foo.bar`, `client.config.x`, or
// the literal `"..."`/`true`/`false`/identifier.
function tsExprToGo(expr, optName) {
  if (!expr) return `""`;
  const t = expr.trim();
  if (t === "undefined" || t === "null") return `""`;
  if (t === "true") return "true";
  if (t === "false") return "false";
  // options?.foo → opt.Name (foo is a key in *SnapOptions)
  const m = t.match(/^options\?\.(\w+)$/);
  if (m) {
    const field = pascalKey(m[1]);
    return `${optName}.${field}`;
  }
  // Plain identifier — assume it's already a Go var (none in current mappings).
  return t;
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

function renderModule(moduleKey) {
  const types = typeDecls.get(moduleKey) ?? new Map();
  const fns = fnDecls.get(moduleKey) ?? [];
  const out = [];

  out.push("// Code generated by scripts/ts2go.mjs — DO NOT EDIT.");
  out.push("//");
  out.push(`// Source: @harizinside/doku/src/products/${moduleKey}.ts`);
  out.push("");
  out.push("package doku");
  out.push("");

  const needsFmt = fns.some((f) => f.params.length > 0);
  out.push("import (");
  out.push("\t\"context\"");
  out.push("\t\"encoding/json\"");
  if (needsFmt) out.push("\t\"fmt\"");
  out.push(")");
  out.push("");

  const needsSnapOptions = fns.some((f) => f.scheme === "snap" || f.scheme === "snapShared");
  const usesSharedKey = fns.some((f) => f.scheme === "snapShared");

  out.push(`type ${pascal(moduleKey)}Service struct{ c *Client }`);
  out.push("");

  if (needsSnapOptions) {
    out.push(`// ${pascal(moduleKey)}SnapOptions configures the SNAP path of the uniform ${moduleKey} module.`);
    out.push(`type ${pascal(moduleKey)}SnapOptions struct {`);
    out.push(`\t// CHANNEL-ID header (some flows require e.g. "H2H")`);
    out.push(`\tChannelID string \`json:"channelId,omitempty"\``);
    if (usesSharedKey) {
      out.push(`\t// B2B2C customer token; sent as \`Authorization-Customer: Bearer ...\` when set`);
      out.push(`\tCustomerToken string \`json:"customerToken,omitempty"\``);
    }
    out.push(`}`);
    out.push("");
  }

  // Render interfaces (with possible recursive named-struct children).
  const rendered = new Set();
  for (const t of types.values()) {
    renderType(t, types, rendered, out);
  }
  if (types.size > 0) out.push("");

  const moduleStructName = pascal(moduleKey) + "Service";
  for (const f of fns) {
    renderFn(f, moduleStructName, types, out);
  }
  return out.join("\n");
}

function renderType(t, allTypes, rendered, out) {
  const resolved = resolveName(t.moduleKey, t.name);
  if (rendered.has(resolved)) return;
  rendered.add(resolved);
  // First, render any nested type-literal structs referenced by name.
  for (const f of t.fields) {
    const refName = extractRefName(f.type ?? "");
    if (!refName) continue;
    // Try to find the named type across all modules (may be in another one).
    for (const [mk, types] of typeDecls) {
      const resolvedRef = resolveName(mk, refName);
      if (types.has(refName) && !rendered.has(resolvedRef)) {
        renderType(types.get(refName), types, rendered, out);
      }
    }
  }
  if (t.jsdoc) out.push(formatDoc(t.jsdoc));
  out.push(`type ${resolved} struct {`);
  for (const f of t.fields) {
    if (f.doc) out.push(formatDoc(f.doc));
    const ref = extractRefName(f.type);
    const type = ref ? f.type.replace(ref, resolveName(t.moduleKey, ref)) : f.type;
    out.push(`\t${f.name} ${type} ${f.tag}`);
  }
  out.push("}");
}

// extract a top-level type name from a Go type expression like "[]Foo" or
// "Foo" or "*Foo".
function extractRefName(goType) {
  let s = goType.replace(/^[\[\]*]+/, "");
  const m = s.match(/^([A-Z]\w*)/);
  return m ? m[1] : null;
}

function formatDoc(text) {
  if (!text) return "";
  const lines = text.split("\n").map((l) => "// " + l.trim()).filter(Boolean);
  return lines.join("\n");
}

function renderFn(f, structName, allTypes, out) {
  if (f.jsdoc) out.push(formatDoc(f.jsdoc));

  const args = ["ctx context.Context"];
  for (const p of f.params) args.push(`${p} string`);
  if (f.bodyArg) {
    args.push(`body ${f.bodyTypeName}`);
  }
  if (f.scheme === "snap" || f.scheme === "snapShared") {
    // The TS `SnapOptions` type lives in the same module — find its Go name.
    const moduleKey = currentGoName(f);
    const optTypeName = `${pascal(moduleKey)}SnapOptions`;
    args.push(`options *${optTypeName}`);
  }
  const returnPair = `(json.RawMessage, error)`;

  out.push(`func (s *${structName}) ${pascal(f.fnName)}(${args.join(", ")}) ${returnPair} {`);

  if (f.scheme === "nonSnap") {
    if (f.bodyArg) {
      out.push(`\tvar out json.RawMessage`);
      out.push(`\tif err := s.c.RequestNonSnap(ctx, ${JSON.stringify(f.httpMethod)}, ${JSON.stringify(f.path)}, body, &out, ""); err != nil {`);
      out.push(`\t\treturn nil, err`);
      out.push(`\t}`);
      out.push(`\treturn out, nil`);
    } else if (f.params.length > 0) {
      out.push(`\tvar out json.RawMessage`);
      out.push(`\tpath := fmt.Sprintf(${JSON.stringify(f.path)}, ${f.params.join(", ")})`);
      out.push(`\tif err := s.c.RequestNonSnap(ctx, ${JSON.stringify(f.httpMethod)}, path, nil, &out, ""); err != nil {`);
      out.push(`\t\treturn nil, err`);
      out.push(`\t}`);
      out.push(`\treturn out, nil`);
    } else {
      out.push(`\tvar out json.RawMessage`);
      out.push(`\tif err := s.c.RequestNonSnap(ctx, ${JSON.stringify(f.httpMethod)}, ${JSON.stringify(f.path)}, nil, &out, ""); err != nil {`);
      out.push(`\t\treturn nil, err`);
      out.push(`\t}`);
      out.push(`\treturn out, nil`);
    }
  } else {
    const moduleKey = currentGoName(f);
    const optName = "options";
    const channelExpr = tsExprToGo(f.channelIdExpr, optName);
    const customerExpr = tsExprToGo(f.customerTokenExpr, optName);
    // Build SnapOptions via locals so a nil *SnapOptions doesn't panic on
    // field access.
    const channelLocal = f.channelIdExpr ? `channelID := "${channelExpr}"` : `channelID := ""`;
    const customerLocal = f.customerTokenExpr ? `customerToken := "${customerExpr}"` : `customerToken := ""`;
    const guardChannel = f.channelIdExpr ? `\t${channelLocal}\n\tif options != nil { channelID = options.ChannelID }` : "";
    const guardCustomer = f.customerTokenExpr ? `\t${customerLocal}\n\tif options != nil { customerToken = options.CustomerToken }` : "";
    const opts = [];
    if (f.bodyArg) opts.push("Body: body");
    if (f.useSharedKey) opts.push("UseSharedKey: true");
    if (f.channelIdExpr) opts.push("ChannelID: channelID");
    if (f.customerTokenExpr) opts.push("CustomerToken: customerToken");
    out.push(`\tvar out json.RawMessage`);
    if (f.channelIdExpr) {
      out.push(`\tchannelID := ""`);
      out.push(`\tif options != nil { channelID = options.ChannelID }`);
    }
    if (f.customerTokenExpr) {
      out.push(`\tcustomerToken := ""`);
      out.push(`\tif options != nil { customerToken = options.CustomerToken }`);
    }
    out.push(`\tif err := s.c.RequestSnap(ctx, ${JSON.stringify(f.httpMethod)}, ${JSON.stringify(f.path)}, SnapOptions{${opts.join(", ")}}, &out); err != nil {`);
    out.push(`\t\treturn nil, err`);
    out.push(`\t}`);
    out.push(`\treturn out, nil`);
  }
  out.push(`}`);
  out.push("");
}

// ---------------------------------------------------------------------------
// Collision resolution
// ---------------------------------------------------------------------------

// Build a global registry of every interface name → set of modules it appears in.
// Collisions get the module name prefix.
const nameRegistry = new Map();
for (const [moduleKey, types] of typeDecls) {
  for (const tname of types.keys()) {
    if (!nameRegistry.has(tname)) nameRegistry.set(tname, new Set());
    nameRegistry.get(tname).add(moduleKey);
  }
}

const RENAMES = new Map();
for (const [tname, modules] of nameRegistry) {
  if (modules.size > 1) {
    for (const m of modules) {
      RENAMES.set(`${m}::${tname}`, pascal(m) + tname);
    }
  }
}

function resolveName(moduleKey, tname) {
  return RENAMES.get(`${moduleKey}::${tname}`) ?? tname;
}

// Update fn bodyType lookups now that RENAMES is populated.
for (const [moduleKey, fns] of fnDecls) {
  for (const f of fns) {
    if (!f.bodyArg) continue;
    f.bodyTypeName = resolveName(moduleKey, pascal(f.fnName) + "Body");
  }
}

// ---------------------------------------------------------------------------
// Emit
// ---------------------------------------------------------------------------

let written = 0;
for (const fileName of UNIFORM) {
  const moduleKey = fileName.replace(/\.ts$/, "");
  const goFile = `zz_${moduleKey.replace(/[A-Z]/g, (c) => "_" + c.toLowerCase()).replace(/^_/, "")}_gen.go`;
  const code = renderModule(moduleKey);
  const target = join(pkgRoot, goFile);
  writeFileSync(target, code);
  written++;
  console.log(`wrote: ${goFile} (${fnDecls.get(moduleKey)?.length ?? 0} functions, ${typeDecls.get(moduleKey)?.size ?? 0} types)`);
}
console.log(`\nDone. ${written} module(s) generated.`);