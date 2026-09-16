import {
  type HTMLAttributes,
  type ReactNode,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import { createPortal } from 'react-dom';

type TriggerProps = Pick<
  HTMLAttributes<HTMLElement>,
  'aria-describedby' | 'onMouseEnter' | 'onMouseLeave' | 'onFocus' | 'onBlur'
>;

export function Tooltip({
  content,
  children,
  enabled = true,
}: {
  content: string;
  children: (props: TriggerProps) => ReactNode;
  enabled?: boolean;
}) {
  const id = useId();
  const popup = useRef<HTMLDivElement>(null);
  const [anchor, setAnchor] = useState<DOMRect | null>(null);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const [dismissed, setDismissed] = useState(false);
  const [position, setPosition] = useState({ top: 0, left: 0 });
  const open = enabled && anchor !== null && (hovered || focused) && !dismissed;

  useLayoutEffect(() => {
    if (!open || !popup.current) return;
    const { width, height } = popup.current.getBoundingClientRect();
    const left = Math.max(8, Math.min(anchor.left, window.innerWidth - width - 8));
    const top =
      anchor.bottom + height <= window.innerHeight
        ? anchor.bottom
        : Math.max(0, anchor.top - height);
    // Content can change while open, so measure after each render and update only if it moved.
    setPosition((previous) =>
      previous.left === left && previous.top === top ? previous : { left, top },
    );
  });

  useEffect(() => {
    if (!open) return;
    const dismiss = () => setDismissed(true);
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') dismiss();
    };
    document.addEventListener('keydown', onKeyDown);
    window.addEventListener('scroll', dismiss, true);
    window.addEventListener('resize', dismiss);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      window.removeEventListener('scroll', dismiss, true);
      window.removeEventListener('resize', dismiss);
    };
  }, [open]);

  return (
    <>
      {children({
        'aria-describedby': open ? id : undefined,
        onMouseEnter: (event) => {
          setAnchor(event.currentTarget.getBoundingClientRect());
          setHovered(true);
          setDismissed(false);
        },
        onMouseLeave: (event) => {
          if (
            !(event.relatedTarget instanceof Node) ||
            !popup.current?.contains(event.relatedTarget)
          ) {
            setHovered(false);
          }
        },
        onFocus: (event) => {
          setAnchor(event.currentTarget.getBoundingClientRect());
          setFocused(true);
          setDismissed(false);
        },
        onBlur: () => setFocused(false),
      })}
      {open &&
        createPortal(
          <div
            ref={popup}
            id={id}
            role="tooltip"
            onMouseLeave={() => setHovered(false)}
            className="fixed z-50 max-w-[min(16rem,calc(100vw-1rem))] py-2"
            style={position}
          >
            <div className="rounded-md border border-hairline bg-surface-raised px-3 py-2 text-xs text-text-primary shadow-lg">
              {content}
            </div>
          </div>,
          document.body,
        )}
    </>
  );
}
