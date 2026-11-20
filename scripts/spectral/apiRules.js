"use strict";

const verbs = new Set([
  "get",
  "create",
  "update",
  "delete",
  "list",
  "search",
  "add",
  "remove",
  "set",
  "post",
  "put",
  "patch",
  "fetch",
  "make",
  "do",
  "cancel",
  "confirm",
]);

const allowedStatus = new Set([
  "200",
  "201",
  "202",
  "204",
  "400",
  "401",
  "403",
  "404",
  "409",
  "412",
  "413",
  "422",
  "429",
  "500",
  "503",
]);

const irregularPlurals = new Set([
  "people",
  "children",
  "data",
  "media",
  "staff",
  "feedback",
  "metadata",
  "criteria",
  "analytics",
  "news",
  "series",
]);

const camel = /^[a-z][a-zA-Z0-9]*$/;
const kebab = /^[a-z0-9]+(-[a-z0-9]+)*$/;
const methods = ["get", "post", "put", "patch", "delete", "head", "options"];
const publicPath = /^\/api\/v[0-9]+\//;

function operations(doc) {
  const paths = (doc && doc.paths) || {};
  const found = [];
  for (const [path, item] of Object.entries(paths)) {
    if (!item || typeof item !== "object" || path.charAt(0) !== "/") {
      continue;
    }
    for (const method of methods) {
      if (item[method] && typeof item[method] === "object") {
        found.push({ path, method, op: item[method], item });
      }
    }
  }
  return found;
}

function parameters(op, item) {
  return [...(item.parameters || []), ...(op.parameters || [])].filter(Boolean);
}

function hasBody(op, item) {
  return parameters(op, item).some((param) => param.in === "body") || Boolean(op.requestBody);
}

function statuses(op) {
  return Object.keys(op.responses || {}).filter((code) => /^[0-9]+$/.test(code));
}

function hasStatus(op, codes) {
  const present = new Set(statuses(op));
  return codes.some((code) => present.has(code));
}

function segments(path) {
  return path.split("/").filter(Boolean);
}

function staticSegments(path) {
  const parts = segments(path);
  const rest = publicPath.test(path) ? parts.slice(2) : parts;
  return rest.filter((segment) => !segment.startsWith("{") && !/^v[0-9]+$/.test(segment));
}

function isOperational(path) {
  return path === "/healthz" || path === "/readyz";
}

function isSSE(op) {
  if (Array.isArray(op.produces) && op.produces.includes("text/event-stream")) {
    return true;
  }
  const ok = op.responses && op.responses["200"];
  return Boolean(ok && ok.content && ok.content["text/event-stream"]);
}

function isWebSocket(op) {
  return op["x-realtime"] === "websocket";
}

// A GET is a collection when its 200 body is a list: an object with an items
// array, or a bare array. Path shape is not used, so /api/v1/me is not a list.
// SSE streams are never collections.
function isCollection(doc, op, path, method) {
  if (method !== "get" || !publicPath.test(path) || isSSE(op)) {
    return false;
  }
  const schema = resolveSchema(doc, op.responses && op.responses["200"] && op.responses["200"].schema);
  if (!schema) {
    return false;
  }
  if (schema.type === "array") {
    return true;
  }
  const items = schema.properties && schema.properties.items;
  return Boolean(items) && items.type === "array";
}

function resolveSchema(doc, schema) {
  if (!schema || typeof schema !== "object") {
    return null;
  }
  if (typeof schema.$ref === "string") {
    const name = schema.$ref.split("/").pop();
    return (doc.definitions && doc.definitions[name]) || null;
  }
  return schema;
}

function walkProperties(node, at, visit) {
  if (!node || typeof node !== "object") {
    return;
  }
  if (Array.isArray(node)) {
    node.forEach((child, index) => walkProperties(child, at.concat(index), visit));
    return;
  }
  if (node.properties && typeof node.properties === "object" && !Array.isArray(node.properties)) {
    for (const key of Object.keys(node.properties)) {
      visit(key, at.concat("properties", key));
    }
  }
  for (const [key, child] of Object.entries(node)) {
    if (key === "properties") {
      continue;
    }
    if (child && typeof child === "object") {
      walkProperties(child, at.concat(key), visit);
    }
  }
}

function pageSizeLimit(param) {
  if (!param) {
    return null;
  }
  if (typeof param.maximum === "number") {
    return param.maximum;
  }
  if (param.schema && typeof param.schema.maximum === "number") {
    return param.schema.maximum;
  }
  return null;
}

function pageSizeType(param) {
  if (!param) {
    return "";
  }
  return param.type || (param.schema && param.schema.type) || "";
}

function isErrorResponse(schema) {
  if (!schema || typeof schema !== "object") {
    return false;
  }
  if (typeof schema.$ref === "string" && schema.$ref.includes("ErrorResponse")) {
    return true;
  }
  const error = schema.properties && schema.properties.error;
  if (!error || typeof error !== "object") {
    return false;
  }
  if (typeof error.$ref === "string" && error.$ref.includes("ErrorBody")) {
    return true;
  }
  const props = error.properties || {};
  return Boolean(props.code && props.message);
}

function hasEnum(param) {
  const values = (param && param.enum) || (param && param.schema && param.schema.enum);
  return Array.isArray(values) && values.length > 0;
}

module.exports = (doc, opts) => {
  const check = opts && opts.check;
  const errors = [];
  const report = (path, message) => errors.push({ path, message });

  for (const entry of operations(doc)) {
    const { path, method, op, item } = entry;
    const location = ["paths", path, method];
    const codes = statuses(op);

    if (check === "path-versioned" && !isOperational(path) && !publicPath.test(path)) {
      report(["paths", path], `${path} must start with /api/vN/`);
    }
    if (check === "path-kebab-case") {
      for (const segment of staticSegments(path)) {
        if (!kebab.test(segment)) {
          report(["paths", path], `${path} segment ${segment} must be kebab-case`);
        }
      }
    }
    if (check === "path-param-camel-case") {
      for (const match of path.matchAll(/\{([^}]+)\}/g)) {
        if (!camel.test(match[1])) {
          report(["paths", path], `${path} parameter ${match[1]} must be camelCase`);
        }
      }
    }
    if (check === "path-no-verbs") {
      for (const segment of staticSegments(path)) {
        if (verbs.has(segment.toLowerCase())) {
          report(["paths", path], `${path} encodes the action ${segment}; use an HTTP method instead`);
        }
      }
    }
    if (check === "path-collection-plural") {
      const parts = segments(path);
      for (let i = 1; i < parts.length; i += 1) {
        const noun = parts[i - 1];
        if (parts[i].startsWith("{") && !noun.startsWith("{") && !noun.endsWith("s") && !irregularPlurals.has(noun)) {
          report(["paths", path], `${parts[i - 1]} before ${parts[i]} should be a plural collection name`);
        }
      }
    }
    if (check === "path-max-depth") {
      const depth = (path.match(/\{[^}]+\}/g) || []).length;
      if (depth > 2) {
        report(["paths", path], `${path} nests more than two resources`);
      }
    }
    if (check === "get-no-body" && method === "get" && hasBody(op, item)) {
      report(location, "GET must not send a request body");
    }
    if (check === "delete-no-body" && method === "delete" && hasBody(op, item)) {
      report(location, "DELETE must not send a request body");
    }
    if (check === "post-success-status" && method === "post" && !hasStatus(op, ["200", "201", "202"])) {
      report(location, "POST must declare 200, 201, or 202");
    }
    if (check === "put-patch-success-status" && (method === "put" || method === "patch") && !hasStatus(op, ["200", "204"])) {
      report(location, "PUT and PATCH must declare 200 or 204");
    }
    if (check === "delete-success-status" && method === "delete" && !hasStatus(op, ["204", "202"])) {
      report(location, "DELETE must declare 204 or 202");
    }
    if (check === "operation-has-4xx" && !isOperational(path) && !codes.some((code) => code.startsWith("4"))) {
      report(location, "declare at least one 4xx response");
    }
    if (check === "path-param-has-404" && path.includes("{") && !codes.includes("404")) {
      report(location, "operations with a path parameter must declare 404");
    }
    if (check === "body-has-400" && hasBody(op, item) && !codes.includes("400")) {
      report(location, "operations with a body must declare 400");
    }
    if (check === "status-code-allowed") {
      for (const code of codes) {
        if (code === "101" && isWebSocket(op)) {
          continue;
        }
        if (!allowedStatus.has(code)) {
          report(location.concat("responses", code), `${code} is not an allowed status code`);
        }
      }
    }
    if (check === "sse-stream-response" && isSSE(op)) {
      if (!codes.includes("200")) {
        report(location, "SSE operations must declare a 200 text/event-stream response");
      }
    }
    if (check === "query-param-camel-case") {
      parameters(op, item).forEach((param, index) => {
        if (param.in === "query" && !camel.test(param.name || "")) {
          report(location.concat("parameters", index), `query parameter ${param.name} must be camelCase`);
        }
      });
    }
    if (check === "operation-id-camel-case" && !camel.test(op.operationId || "")) {
      report(location.concat("operationId"), "operationId must be camelCase");
    }
    if (check === "operation-tags" && (!Array.isArray(op.tags) || op.tags.length === 0)) {
      report(location.concat("tags"), "operation must declare a tag");
    }
    if (check === "operation-summary" && (typeof op.summary !== "string" || op.summary.trim() === "")) {
      report(location.concat("summary"), "operation must declare a summary");
    }
    if (check === "error-response-ref") {
      for (const code of codes) {
        if (Number(code) < 400) {
          continue;
        }
        const schema = op.responses[code] && op.responses[code].schema;
        if (!isErrorResponse(schema)) {
          report(location.concat("responses", code), `${code} must use the ErrorResponse schema`);
        }
      }
    }
    if (check === "collection-pagination-params" && isCollection(doc, op, path, method)) {
      const query = parameters(op, item).filter((param) => param.in === "query");
      const pageSize = query.find((param) => param.name === "pageSize");
      const pageToken = query.find((param) => param.name === "pageToken");
      if (!pageSize || !pageToken) {
        report(location, "list operations must declare pageSize and pageToken");
      }
      if (pageSize && (pageSizeType(pageSize) !== "integer" || pageSizeLimit(pageSize) === null || pageSizeLimit(pageSize) > 100)) {
        report(location, "pageSize must be an integer with maximum <= 100");
      }
    }
    if (check === "collection-pagination-response" && isCollection(doc, op, path, method)) {
      const schema = resolveSchema(doc, op.responses && op.responses["200"] && op.responses["200"].schema);
      const props = (schema && schema.properties) || {};
      const items = props.items;
      if (!items || items.type !== "array" || !Object.prototype.hasOwnProperty.call(props, "nextPageToken")) {
        report(location.concat("responses", "200"), "list responses must include items and nextPageToken");
      }
    }
    if (check === "sort-by-enum") {
      parameters(op, item).forEach((param, index) => {
        if (param.in === "query" && param.name === "sortBy" && !hasEnum(param)) {
          report(location.concat("parameters", index), "sortBy must declare an enum of supported fields");
        }
      });
    }
  }

  if (check === "property-camel-case") {
    walkProperties(doc, [], (key, path) => {
      if (!camel.test(key)) {
        report(path, `property ${key} must be camelCase`);
      }
    });
  }

  return errors;
};
