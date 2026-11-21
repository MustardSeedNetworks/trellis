import { Command } from 'cmdk';
import { Languages, Search } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { languages } from '@/i18n';
import { iconSizes } from '../constants/sizes';
import { useNavGroups } from '../navGroups';

const GROUP =
  '[&_[cmdk-group-heading]]:px-3 [&_[cmdk-group-heading]]:py-1 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-wider [&_[cmdk-group-heading]]:text-text-muted';

const ITEM =
  'flex min-h-11 cursor-pointer items-center gap-3 rounded-[11px] px-3 py-2 text-text-primary aria-selected:bg-surface-hover';

/**
 * Keyboard-first palette: Cmd+K on macOS, Ctrl+K elsewhere. It lists every
 * route the rail offers and the actions that change how the tool looks rather
 * than what it holds. The theme switch is not here on purpose: useTheme keeps
 * its state per call site, so a second switch would leave the rail's showing
 * the wrong destination.
 */
export function CommandPalette() {
  const { t, i18n } = useTranslation('common');
  const navigate = useNavigate();
  const navGroups = useNavGroups();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpen((value) => !value);
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, []);

  function close() {
    setOpen(false);
    setQuery('');
  }

  function run(action: () => void) {
    action();
    close();
  }

  return (
    <Command.Dialog
      open={open}
      onOpenChange={(next) => (next ? setOpen(true) : close())}
      label={t('commandPalette.dialogLabel')}
      className="fixed inset-0 z-[60] flex items-start justify-center px-4 pt-[10vh]"
    >
      <button
        type="button"
        onClick={close}
        aria-label={t('commandPalette.closeAriaLabel')}
        className="absolute inset-0 bg-scrim/60"
      />
      <div
        data-testid="command-palette"
        className="panel relative w-full max-w-xl overflow-hidden shadow-2xl"
      >
        <div className="flex items-center gap-3 border-b border-hairline px-4 py-3">
          <Search className={`${iconSizes.md} text-text-muted`} aria-hidden />
          <Command.Input
            autoFocus
            value={query}
            onValueChange={setQuery}
            placeholder={t('commandPalette.inputPlaceholder')}
            className="min-w-0 flex-1 bg-transparent text-sm text-text-primary placeholder:text-text-muted focus:outline-none"
          />
          <kbd className="hidden rounded border border-hairline px-1.5 py-0.5 text-[11px] text-text-muted sm:inline-flex">
            {t('commandPalette.escKey')}
          </kbd>
        </div>
        <Command.List className="max-h-[60vh] overflow-y-auto p-2 text-sm">
          <Command.Empty className="px-3 py-6 text-center text-text-muted">
            {t('commandPalette.noMatches')}
          </Command.Empty>

          <Command.Group heading={t('commandPalette.navigate')} className={GROUP}>
            {navGroups
              .flatMap((group) => group.items)
              .map((item) => (
                <Command.Item
                  key={item.path}
                  value={item.label}
                  onSelect={() => run(() => navigate(item.path))}
                  data-testid={`palette-${item.path === '/' ? 'surveys' : item.path.slice(1)}`}
                  className={ITEM}
                >
                  <item.icon className={`${iconSizes.md} text-text-muted`} aria-hidden />
                  <span>{item.label}</span>
                </Command.Item>
              ))}
          </Command.Group>

          <Command.Group heading={t('commandPalette.actions')} className={GROUP}>
            {languages
              .filter((language) => language.code !== i18n.resolvedLanguage)
              .map((language) => {
                const label = t('commandPalette.switchLanguage', {
                  language: language.nativeLabel,
                });
                return (
                  <Command.Item
                    key={language.code}
                    value={label}
                    onSelect={() => run(() => void i18n.changeLanguage(language.code))}
                    data-testid={`palette-language-${language.code}`}
                    className={ITEM}
                  >
                    <Languages className={`${iconSizes.md} text-text-muted`} aria-hidden />
                    <span>{label}</span>
                  </Command.Item>
                );
              })}
          </Command.Group>
        </Command.List>
      </div>
    </Command.Dialog>
  );
}
