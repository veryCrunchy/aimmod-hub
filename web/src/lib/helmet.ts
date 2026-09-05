import * as HelmetAsyncModule from "react-helmet-async";
import { Children, cloneElement, createElement, isValidElement, type ComponentProps, type ReactNode } from "react";

type HelmetAsyncExports = typeof import("react-helmet-async");

const helmetAsync = HelmetAsyncModule as HelmetAsyncExports & {
  default?: Partial<HelmetAsyncExports>;
};

const NativeHelmet = (helmetAsync.Helmet ?? helmetAsync.default?.Helmet) as HelmetAsyncExports["Helmet"];
export function Helmet(props: ComponentProps<typeof NativeHelmet>) {
  // React 19 requires a title's children to be one string, not JSX's interpolation array.
  // Preserve legacy pages' specific titles while using Helmet's native metadata support.
  const children = Children.map(props.children, child => {
    if (!isValidElement<{ children?: ReactNode }>(child) || child.type !== "title") return child;
    return cloneElement(child, {}, Children.toArray(child.props.children).map(String).join(""));
  });
  return createElement(NativeHelmet, { ...props, children });
}
export const HelmetProvider = (helmetAsync.HelmetProvider ?? helmetAsync.default?.HelmetProvider) as HelmetAsyncExports["HelmetProvider"];
export const HelmetData = (helmetAsync.HelmetData ?? helmetAsync.default?.HelmetData) as HelmetAsyncExports["HelmetData"];

