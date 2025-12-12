import type { ComponentProps } from "react";

import { cn } from "@/lib/utils";

interface BaseNodeProps extends ComponentProps<"div"> {
  selected?: boolean;
}

export function BaseNode({ className, selected, ...props }: BaseNodeProps) {
  return (
    <div
      data-selected={selected || undefined}
      className={cn(
        // Industrial control room styling
        "group bg-card text-card-foreground relative rounded border",
        "border-[oklch(0.35_0.02_260)]",
        "transition-[border-color,box-shadow] duration-150",
        // Hover glow effect - amber
        "hover:border-[oklch(0.78_0.18_75)] hover:shadow-[0_0_12px_oklch(0.78_0.18_75_/_0.2)]",
        // Selected state - same amber as hover
        selected && "border-[oklch(0.78_0.18_75)] shadow-[0_0_16px_oklch(0.78_0.18_75_/_0.3)]",
        className,
      )}
      tabIndex={0}
      {...props}
    />
  );
}

/**
 * A container for a consistent header layout intended to be used inside the
 * `<BaseNode />` component.
 */
export function BaseNodeHeader({
  className,
  showBorder = true,
  style,
  ...props
}: ComponentProps<"header"> & { showBorder?: boolean }) {
  return (
    <header
      {...props}
      style={style}
      className={cn(
        "flex flex-row items-center justify-between gap-2 px-3 py-2",
        showBorder && "border-b border-[oklch(0.28_0.02_260)]",
        className,
      )}
    />
  );
}

/**
 * The title text for the node. To maintain a native application feel, the title
 * text is not selectable.
 */
export function BaseNodeHeaderTitle({
  className,
  ...props
}: ComponentProps<"h3">) {
  return (
    <h3
      data-slot="base-node-title"
      className={cn("select-none flex-1 font-semibold text-sm", className)}
      {...props}
    />
  );
}

export function BaseNodeContent({
  className,
  ...props
}: ComponentProps<"div">) {
  return (
    <div
      data-slot="base-node-content"
      className={cn("flex flex-col gap-y-1 px-3 py-2", className)}
      {...props}
    />
  );
}

export function BaseNodeFooter({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="base-node-footer"
      className={cn(
        "flex flex-col items-center gap-y-2 border-t border-[oklch(0.28_0.02_260)] px-3 pt-2 pb-2",
        className,
      )}
      {...props}
    />
  );
}
