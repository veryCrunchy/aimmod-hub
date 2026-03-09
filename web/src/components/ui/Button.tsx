import type { AnchorHTMLAttributes, ButtonHTMLAttributes, ReactNode } from "react";
import { Link, type LinkProps } from "react-router-dom";
import { cn } from "../../lib/cn";

const buttonStyles = {
  base: "inline-flex min-h-11 items-center justify-center rounded-full border px-4 text-sm transition-colors",
  primary: "border-mint/35 bg-mint/12 text-mint hover:border-mint/55 hover:bg-mint/18",
  secondary: "border-line bg-white/4 text-text hover:border-line-strong hover:bg-white/6"
} as const;

type ButtonVariant = keyof typeof buttonStyles;

type CommonProps = {
  children: ReactNode;
  className?: string;
  variant?: ButtonVariant;
};

type ButtonAsLinkProps = CommonProps &
  Omit<LinkProps, "className" | "children"> & {
    to: LinkProps["to"];
    href?: never;
  };

type ButtonAsAnchorProps = CommonProps &
  Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "className" | "children"> & {
    href: string;
    to?: never;
  };

type ButtonAsButtonProps = CommonProps &
  Omit<ButtonHTMLAttributes<HTMLButtonElement>, "className" | "children"> & {
    to?: never;
    href?: never;
  };

function classes(variant: ButtonVariant, className?: string) {
  return cn(buttonStyles.base, buttonStyles[variant], className);
}

export function Button(props: ButtonAsLinkProps | ButtonAsAnchorProps | ButtonAsButtonProps) {
  const variant = props.variant ?? "secondary";

  if ("to" in props && props.to !== undefined) {
    const { className, children, variant: _variant, ...rest } = props as ButtonAsLinkProps;
    return (
      <Link {...rest} className={classes(variant, className)}>
        {children}
      </Link>
    );
  }

  if ("href" in props && props.href !== undefined) {
    const { className, children, variant: _variant, ...rest } = props as ButtonAsAnchorProps;
    return (
      <a {...rest} className={classes(variant, className)}>
        {children}
      </a>
    );
  }

  const { className, children, variant: _variant, ...rest } = props as ButtonAsButtonProps;
  return (
    <button {...rest} className={classes(variant, className)}>
      {children}
    </button>
  );
}
