import * as HelmetAsyncModule from "react-helmet-async";

type HelmetAsyncExports = typeof import("react-helmet-async");

const helmetAsync = HelmetAsyncModule as HelmetAsyncExports & {
  default?: Partial<HelmetAsyncExports>;
};

export const Helmet = (helmetAsync.Helmet ?? helmetAsync.default?.Helmet) as HelmetAsyncExports["Helmet"];
export const HelmetProvider = (helmetAsync.HelmetProvider ?? helmetAsync.default?.HelmetProvider) as HelmetAsyncExports["HelmetProvider"];
export const HelmetData = (helmetAsync.HelmetData ?? helmetAsync.default?.HelmetData) as HelmetAsyncExports["HelmetData"];

