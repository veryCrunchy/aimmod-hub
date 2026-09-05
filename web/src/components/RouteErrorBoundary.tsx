import { Component, type PropsWithChildren } from "react";
import { RefreshCw } from "lucide-react";
import { useLocation } from "react-router-dom";
import { isRouteLoadError } from "../lib/routeLoadError";
import { Button } from "./ui/Button";

class PageErrorBoundary extends Component<PropsWithChildren, { failed: boolean; loadError: boolean }> {
  state = { failed: false, loadError: false };

  static getDerivedStateFromError(error: unknown) {
    return { failed: true, loadError: isRouteLoadError(error) };
  }

  render() {
    if (!this.state.failed) return this.props.children;
    const offline = typeof navigator !== "undefined" && navigator.onLine === false;
    return <section role="alert" className="flex min-h-80 flex-col items-center justify-center gap-4 px-6 py-12 text-center">
      <h1 className="text-2xl">{offline ? "You're offline" : this.state.loadError ? "This page needs to reload" : "This page couldn't open"}</h1>
      <p className="max-w-md text-sm leading-6 text-muted">{offline ? "Reconnect, then reload this page to continue."
        : this.state.loadError ? "A new version may be available. Reload to continue on this page."
        : "Please reload and try again."}</p>
      <Button variant="primary" onClick={() => window.location.reload()}><RefreshCw size={16} aria-hidden="true" />Reload page</Button>
    </section>;
  }
}

export function RouteErrorBoundary({ children }: PropsWithChildren) {
  const location = useLocation();
  // A failed lazy module stays rejected until a document reload. Other routes
  // must remain navigable without carrying the failed route's boundary state.
  return <PageErrorBoundary key={location.pathname}>{children}</PageErrorBoundary>;
}
