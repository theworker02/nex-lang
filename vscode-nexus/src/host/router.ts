/**
 * Lightweight HTTP router with chi-style `{param}` segments.
 */

export type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export interface RouteMatch {
  params: Record<string, string>;
  handler: RouteHandler;
}

export type RouteHandler = (
  req: IncomingRequest,
) => unknown | Promise<unknown>;

export interface IncomingRequest {
  method: string;
  path: string;
  query: Record<string, string>;
  headers: Record<string, string>;
  params: Record<string, string>;
  cookies: Record<string, string>;
  form: Record<string, string>;
  body: string;
  requestId: string;
  user: Record<string, unknown> | null;
  authVia: string;
}

interface RouteEntry {
  method: HttpMethod;
  pattern: string;
  parts: Part[];
  handler: RouteHandler;
  requireAuth: boolean;
}

type Part =
  | { kind: "lit"; value: string }
  | { kind: "param"; name: string };

function parsePattern(pattern: string): Part[] {
  const trimmed = pattern.replace(/\/+$/, "") || "/";
  const segs = trimmed === "/" ? [""] : trimmed.split("/");
  return segs.map((seg) => {
    if (seg.startsWith("{") && seg.endsWith("}")) {
      return { kind: "param", name: seg.slice(1, -1) };
    }
    return { kind: "lit", value: seg };
  });
}

function matchParts(
  parts: Part[],
  path: string,
): Record<string, string> | null {
  const normalized = path.replace(/\/+$/, "") || "/";
  const segs = normalized === "/" ? [""] : normalized.split("/");
  if (segs.length !== parts.length) {
    return null;
  }
  const params: Record<string, string> = {};
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i]!;
    const seg = segs[i]!;
    if (part.kind === "lit") {
      if (part.value !== seg) {
        return null;
      }
    } else {
      params[part.name] = decodeURIComponent(seg);
    }
  }
  return params;
}

export function pathRequiresAuth(method: string, path: string): boolean {
  return (
    path.includes("/settings") ||
    path.startsWith("/getting-started") ||
    path.startsWith("/api/user") ||
    path.startsWith("/orgs/new") ||
    path.startsWith("/invites/") ||
    (method !== "GET" &&
      (path.includes("/owners") ||
        path.includes("/members") ||
        path.includes("/teams") ||
        path.includes("/transfer") ||
        path.startsWith("/api/v1/orgs") ||
        path.startsWith("/orgs/"))) ||
    path === "/api/v1/publish" ||
    path === "/api/v1/trusted-publishing/token" ||
    path.startsWith("/admin") ||
    path.includes("/yank") ||
    path.includes("/unyank") ||
    path.includes("/unpublish")
  );
}

export class Router {
  private readonly routes: RouteEntry[] = [];
  notFoundHandler: RouteHandler | null = null;

  get routeCount(): number {
    return this.routes.length;
  }

  add(method: HttpMethod, pattern: string, handler: RouteHandler): void {
    this.routes.push({
      method,
      pattern,
      parts: parsePattern(pattern),
      handler,
      requireAuth: pathRequiresAuth(method, pattern),
    });
  }

  match(method: string, path: string): (RouteMatch & { requireAuth: boolean }) | null {
    const m = method.toUpperCase() as HttpMethod;
    for (const route of this.routes) {
      if (route.method !== m) {
        continue;
      }
      const params = matchParts(route.parts, path);
      if (params) {
        return { params, handler: route.handler, requireAuth: route.requireAuth };
      }
    }
    return null;
  }
}
