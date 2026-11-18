import { describe, it, expect } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { createVyzorI18n, changeVyzorLanguage, VYZOR_I18N_DEFAULT_NS } from '@/lib/i18n';
import { I18nextProvider, useTranslation } from 'react-i18next';
import { vyzorCommonEn } from '@/lib/i18n/locales/en/vyzor-common-en';
import { vyzorCommonFr } from '@/lib/i18n/locales/fr/vyzor-common-fr';

function Probe() {
  const { t } = useTranslation(VYZOR_I18N_DEFAULT_NS);
  return <span data-testid="out">{t('prefs.language')}</span>;
}

describe('i18n', () => {
  it('en and fr expose the same keys', () => {
    const en = Object.keys(vyzorCommonEn).sort();
    const fr = Object.keys(vyzorCommonFr).sort();
    const missingInFr = en.filter((k) => !(k in vyzorCommonFr));
    const extraInFr = fr.filter((k) => !(k in vyzorCommonEn));
    expect(missingInFr).toEqual([]);
    expect(extraInFr).toEqual([]);
  });

  it('re-renders translated text when the language changes', async () => {
    const i18n = createVyzorI18n();
    render(
      <I18nextProvider i18n={i18n}>
        <Probe />
      </I18nextProvider>,
    );
    expect(screen.getByTestId('out').textContent).toBe('Interface language');

    await act(async () => {
      await changeVyzorLanguage(i18n, 'fr');
    });
    expect(screen.getByTestId('out').textContent).toBe("Langue de l'interface");
  });
});
