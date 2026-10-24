"use client";

import * as React from "react";
import * as SliderPrimitive from "@radix-ui/react-slider";
import { cn } from "@/lib/utils";

const Slider = React.forwardRef<
  React.ElementRef<typeof SliderPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof SliderPrimitive.Root>
>(({ className, ...props }, ref) => (
  <SliderPrimitive.Root
    ref={ref}
    data-theme-ui="slider-root"
    className={cn("relative flex w-full touch-none select-none items-center", className)}
    {...props}
  >
    {/*
      轨道与已填充段由 CSS 变量驱动，用于把颜色映射到滑杆上（色相条）。
    */}
    <SliderPrimitive.Track
      data-theme-ui="slider-track"
      className="relative h-2 w-full grow overflow-hidden rounded-full bg-muted"
      style={{ backgroundImage: "var(--fc-slider-track-image, none)" }}
    >
      <SliderPrimitive.Range
        data-theme-ui="slider-range"
        className="absolute h-full bg-primary"
        style={{ display: "var(--fc-slider-range-display, block)" }}
      />
    </SliderPrimitive.Track>
    <SliderPrimitive.Thumb data-theme-ui="slider-thumb" className="block h-5 w-5 rounded-full border-2 border-primary bg-background shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50" />
  </SliderPrimitive.Root>
));
Slider.displayName = SliderPrimitive.Root.displayName;

export { Slider };
