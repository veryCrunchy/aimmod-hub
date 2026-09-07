import "./pages/osuHelp.css";
import React from "react";
import { hydrateRoot, createRoot } from "react-dom/client";
import { HelmetProvider } from "./lib/helmet";
import { BrowserRouter } from "react-router-dom";
import { App } from "./App";
import "./styles.css";

const rootElement = document.getElementById("root");

if (!rootElement) {
  throw new Error("Missing #root element");
}

// The server head serves crawlers; React 19 owns the active head after hydration/mount.
// Remove only the initial SEO nodes so they cannot survive navigation as stale duplicates.
document.head.querySelectorAll('title, meta[name="description"], meta[name="robots"], meta[property^="og:"], meta[name^="twitter:"], link[rel="canonical"], script[type="application/ld+json"]').forEach(node => node.remove());

const app = (
  <React.StrictMode>
    <HelmetProvider>
      <App RouterComponent={BrowserRouter} />
    </HelmetProvider>
  </React.StrictMode>
);

if (rootElement.hasChildNodes()) {
  hydrateRoot(rootElement, app);
} else {
  createRoot(rootElement).render(app);
}
