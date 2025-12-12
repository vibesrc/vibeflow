import type { ComponentProps } from "react";
import { Handle, type HandleProps } from "@xyflow/react";

import { cn } from "@/lib/utils";

export type BaseHandleProps = HandleProps;

export function BaseHandle({
  className,
  children,
  ...props
}: ComponentProps<typeof Handle>) {
  const isInput = props.type === "target";

  return (
    <Handle
      {...props}
      className={cn(
        // Industrial handle styling
        "!h-3 !w-3 !rounded-full !border-2",
        "!bg-secondary !border-border",
        "transition-colors duration-150",
        // Input handles (left) - cyan accent
        isInput && "hover:!border-cyan hover:!bg-cyan/20",
        // Output handles (right) - amber/primary accent
        !isInput && "hover:!border-primary hover:!bg-primary/20",
        className,
      )}
    >
      {children}
    </Handle>
  );
}
