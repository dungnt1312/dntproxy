import { useTheme } from "next-themes"
import { Toaster as Sonner, ToasterProps } from "sonner"
import { useSyncExternalStore } from "react"

const emptySubscribe = () => () => {}

function useIsMounted() {
  return useSyncExternalStore(emptySubscribe, () => true, () => false)
}

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme = "system" } = useTheme()
  const mounted = useIsMounted()

  return (
    <Sonner
      theme={mounted ? (theme as ToasterProps["theme"]) : "system"}
      className="toaster group"
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
        } as React.CSSProperties
      }
      {...props}
    />
  )
}

export { Toaster }
