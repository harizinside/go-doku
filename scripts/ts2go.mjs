#!/usr/bin/env node
/**
 * Generates the uniform product packages (<pkg>/<pkg>_gen.go) from the TS
 * sources of @harizinside/doku, using the TypeScript compiler API installed
 * in that repo (no extra deps; the Go module stays stdlib-only).
 *
 * Hand-written packages (checkout, creditcard, emoney, misc, settlement, va)
 * are never touched.
 *
 * Usage: node scripts/ts2go.mjs [path/to/doku] && gofmt -w .
 */
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const dokuRoot = resolve(process.argv[2] ?? join(root, "..", "doku"));
const tsPath = join(dokuRoot, "node_modules", "typescript", "lib", "typescript.js");
if (!existsSync(tsPath)) {
  console.error(`typescript not found at ${tsPath} — run \`npm install\` in ${dokuRoot}`);
  process.exit(1);
}
const ts = (await import(pathToFileURL(tsPath).href)).default;

/** TS module → API name used in the Go package doc. */
const MODULES = {
  o2o: "O2O (convenience store) payments",
  directDebitLegacy: "BRI Direct Debit (legacy, Non-SNAP)",
  directDebitSnap: "SNAP Direct Debit (OVO, CIMB, Allo, BRI, Mandiri, DANA, ShopeePay)",
  paylater: "Paylater",
  directTransfer: "BCA Direct Transfer",
  kki: "KKI (Kartu Kredit Indonesia)",
  subAccountV1: "Sub Account v1 (Wallet-as-a-Service)",
  subAccountV2: "Sub Account v2 (Wallet-as-a-Service)",
};

const TODO_DOC = "TODO: verify against developers.doku.com";
const INITIALISMS = new Set(["id", "url", "api", "otp", "uuid", "json", "http", "ip"]);

const pascal = (s) => s[0].toUpperCase() + s.slice(1);

/** `invoice_number` → `InvoiceNumber`, `successRegistrationUrl` → `SuccessRegistrationURL`. */
function goName(key) {
  return key
    .split(/[_\s-]+/)
    .flatMap((seg) => seg.replace(/([a-z0-9])([A-Z])/g, "$1 $2").split(" "))
    .filter(Boolean)
    .map((w) => (INITIALISMS.has(w.toLowerCase()) ? w.toUpperCase() : pascal(w)))
    .join("");
}

/** Go local variable name (`accountId` → `accountID`). */
function goLocal(name) {
  const g = goName(name);
  return g.toUpperCase() === g ? g.toLowerCase() : g[0].toLowerCase() + g.slice(1);
}

function docOf(node) {
  const c = node.jsDoc?.[0]?.comment;
  const text = (typeof c === "string" ? c : (c ?? []).map((p) => p.text).join("")).trim();
  return text === TODO_DOC ? "" : text;
}

const comment = (text, indent = "") =>
  text ? text.split("\n").map((l) => `${indent}// ${l.trim()}`.trimEnd()).join("\n") + "\n" : "";

function goType(node) {
  switch (node.kind) {
    case ts.SyntaxKind.StringKeyword: return "string";
    case ts.SyntaxKind.NumberKeyword: return "float64";
    case ts.SyntaxKind.BooleanKeyword: return "*bool"; // pointer so `false` survives omitempty
    case ts.SyntaxKind.ArrayType: return `[]${goType(node.elementType)}`;
    case ts.SyntaxKind.ParenthesizedType: return goType(node.type);
    case ts.SyntaxKind.LiteralType:
      return ts.isStringLiteral(node.literal) ? "string" : ts.isNumericLiteral(node.literal) ? "float64" : "any";
    case ts.SyntaxKind.UnionType: {
      const types = [...new Set(node.types.map(goType))];
      return types.length === 1 ? types[0] : "any";
    }
    case ts.SyntaxKind.TypeReference: {
      const name = node.typeName.getText();
      if (name === "Array") return `[]${node.typeArguments ? goType(node.typeArguments[0]) : "any"}`;
      if (name === "Record") return "map[string]any";
      return name;
    }
    case ts.SyntaxKind.TypeLiteral: return "map[string]any"; // only reached outside a property
    default: return "any";
  }
}

/** Collects `type X struct` decls; inline object literals become `<Parent><Field>`. */
function collectStruct(types, name, members, doc) {
  const decl = { name, doc, fields: [] };
  types.set(name, decl); // inserted before children so parents print first
  for (const m of members) {
    if (!ts.isPropertySignature(m)) continue; // index signatures are dropped
    const json = m.name.getText().replace(/^["']|["']$/g, "");
    const opt = !!m.questionToken;
    const field = { name: goName(json), doc: docOf(m), tag: `json:"${json}${opt ? ",omitempty" : ""}"` };
    const isArr = ts.isArrayTypeNode(m.type);
    const el = isArr ? m.type.elementType : m.type;
    if (ts.isTypeLiteralNode(el)) {
      const sub = name + field.name;
      collectStruct(types, sub, el.members, "");
      field.type = isArr ? `[]${sub}` : opt ? `*${sub}` : sub;
    } else {
      field.type = goType(m.type);
    }
    decl.fields.push(field);
  }
}

/** Go expression for the request path; template params go through url.PathEscape. */
function goPath(node) {
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return JSON.stringify(node.text);
  if (ts.isTemplateExpression(node)) {
    const parts = node.head.text ? [JSON.stringify(node.head.text)] : [];
    for (const span of node.templateSpans) {
      parts.push(`url.PathEscape(${goLocal(span.expression.getText())})`);
      if (span.literal.text) parts.push(JSON.stringify(span.literal.text));
    }
    return parts.join(" + ");
  }
  throw new Error(`unsupported path: ${node.getText()}`);
}

function findRequestCall(node) {
  let found;
  const visit = (n) => {
    if (found) return;
    if (ts.isCallExpression(n) && ts.isPropertyAccessExpression(n.expression)
        && /^request(NonSnap|Snap)$/.test(n.expression.name.text)) found = n;
    else ts.forEachChild(n, visit);
  };
  visit(node);
  return found;
}

function collectFunction(node) {
  const call = findRequestCall(node.body);
  if (!call) throw new Error(`${node.name.text}: no request call`);
  const [methodArg, pathArg, third] = call.arguments;
  const fn = {
    name: goName(node.name.text),
    doc: docOf(node),
    method: methodArg.text,
    path: goPath(pathArg),
    snap: call.expression.name.text === "requestSnap",
    pathParams: [],
    bodyType: null,
    options: false,
    useSharedKey: false,
  };
  for (const p of node.parameters.slice(1)) { // [0] is the client
    const name = p.name.getText();
    if (name === "body") fn.bodyType = p.type.getText();
    else if (name === "options") fn.options = true;
    else fn.pathParams.push(goLocal(name));
  }
  if (fn.snap && third && ts.isObjectLiteralExpression(third)) {
    for (const prop of third.properties) {
      if (prop.name?.getText() === "useSharedKey") fn.useSharedKey = prop.initializer.kind === ts.SyntaxKind.TrueKeyword;
    }
  }
  return fn;
}

function render(moduleKey, pkg, file, sf) {
  const types = new Map();
  const fns = [];
  let snapOpts = null;
  for (const stmt of sf.statements) {
    if (ts.isInterfaceDeclaration(stmt)) {
      if (stmt.name.text.endsWith("SnapOptions")) snapOpts = stmt;
      else collectStruct(types, stmt.name.text, stmt.members, docOf(stmt));
    } else if (ts.isFunctionDeclaration(stmt) && stmt.name) {
      fns.push(collectFunction(stmt));
    }
  }
  const api = MODULES[moduleKey];

  const imports = ['"context"', '"encoding/json"'];
  if (fns.some((f) => f.pathParams.length)) imports.push('"net/url"');
  imports.push("", 'doku "github.com/harizinside/go-doku"');

  let out = `// Code generated by scripts/ts2go.mjs from @harizinside/doku src/products/${file}. DO NOT EDIT.\n\n`;
  out += `// Package ${pkg} is the DOKU ${api} API.\n//\n`;
  out += `// Request types are inferred from DOKU's Postman examples and responses are\n`;
  out += `// returned as raw JSON; verify fields against developers.doku.com.\n`;
  out += `package ${pkg}\n\nimport (\n${imports.map((i) => (i ? `\t${i}` : "")).join("\n")}\n)\n\n`;
  out += `// Service calls the ${api} endpoints.\ntype Service struct{ c *doku.Client }\n\n`;
  out += `// New returns the service bound to c.\nfunc New(c *doku.Client) *Service { return &Service{c: c} }\n\n`;

  if (snapOpts) {
    const fields = snapOpts.members.filter(ts.isPropertySignature)
      .map((m) => ({ name: goName(m.name.getText()), doc: docOf(m) }));
    out += `// SnapOptions sets optional SNAP request headers.\ntype SnapOptions struct {\n`;
    for (const f of fields) out += comment(f.doc, "\t") + `\t${f.name} string\n`;
    out += `}\n\n`;
    out += `func (o *SnapOptions) request(body any, useSharedKey bool) doku.SnapOptions {\n`;
    out += `\topt := doku.SnapOptions{Body: body, UseSharedKey: useSharedKey}\n\tif o != nil {\n`;
    for (const f of fields) out += `\t\topt.${f.name} = o.${f.name}\n`;
    out += `\t}\n\treturn opt\n}\n\n`;
  }

  for (const t of types.values()) {
    out += comment(t.doc) + `type ${t.name} struct {\n`;
    for (const f of t.fields) out += comment(f.doc, "\t") + `\t${f.name} ${f.type} \`${f.tag}\`\n`;
    out += `}\n\n`;
  }

  for (const f of fns) {
    const args = ["ctx context.Context", ...f.pathParams.map((p) => `${p} string`)];
    if (f.bodyType) args.push(`body ${f.bodyType}`);
    if (f.options) args.push("options *SnapOptions");
    const body = f.bodyType ? "body" : "nil";
    const [title, endpoint] = f.doc.replace(/\$\{(\w+)\}/g, "{$1}").split(" — ");
    const label = goName(title.replace(/[^\w\s]/g, " ")) === f.name ? "" : ` (${title})`;
    out += endpoint ? `// ${f.name} calls ${endpoint}${label}.\n` : comment(f.doc);
    out += `func (s *Service) ${f.name}(${args.join(", ")}) (json.RawMessage, error) {\n\tvar out json.RawMessage\n`;
    if (f.snap) {
      const opts = f.options
        ? `options.request(${body}, ${f.useSharedKey})`
        : `doku.SnapOptions{Body: ${body}, UseSharedKey: ${f.useSharedKey}}`;
      out += `\terr := s.c.RequestSnap(ctx, "${f.method}", ${f.path}, ${opts}, &out)\n`;
    } else {
      out += `\terr := s.c.RequestNonSnap(ctx, "${f.method}", ${f.path}, ${body}, &out, "")\n`;
    }
    out += `\treturn out, err\n}\n\n`;
  }
  return out.trimEnd() + "\n";
}

for (const moduleKey of Object.keys(MODULES)) {
  const file = `${moduleKey}.ts`;
  const pkg = moduleKey.toLowerCase();
  const path = join(dokuRoot, "src", "products", file);
  const sf = ts.createSourceFile(path, readFileSync(path, "utf8"), ts.ScriptTarget.ES2022, true);
  mkdirSync(join(root, pkg), { recursive: true });
  writeFileSync(join(root, pkg, `${pkg}_gen.go`), render(moduleKey, pkg, file, sf));
  console.log(`wrote ${pkg}/${pkg}_gen.go`);
}
