import { browser } from '$app/environment';
import { writable } from 'svelte/store';
import {
  DEFAULT_UI_LANGUAGE,
  translations,
  type TranslationKey,
  type UiLanguage
} from '$lib/i18n/translations';

const PARAM_REGEX = /\{\{(\w+)\}\}/g;
const UI_LANGUAGE_STORAGE_KEY = 'zwerffiets-ui-language';

const isUiLanguage = (value: string | null): value is UiLanguage => {
  return value === 'nl' || value === 'en';
};

const getInitialLanguage = (): UiLanguage => {
  if (!browser) {
    return DEFAULT_UI_LANGUAGE;
  }

  const storedLanguage = localStorage.getItem(UI_LANGUAGE_STORAGE_KEY);
  if (isUiLanguage(storedLanguage)) {
    return storedLanguage;
  }

  // Fallback to browser preference if no stored language
  if (navigator.language.toLowerCase().startsWith('nl')) {
    return 'nl';
  }

  return 'en';
};

export const uiLanguage = writable<UiLanguage>(getInitialLanguage());

if (browser) {
  uiLanguage.subscribe((language) => {
    localStorage.setItem(UI_LANGUAGE_STORAGE_KEY, language);
  });
}

export const t = (
  language: UiLanguage,
  key: TranslationKey | string,
  params?: Record<string, string | number>
): string => {
  const dictionary = translations[language] ?? translations[DEFAULT_UI_LANGUAGE];
  const fallback = translations.en as Record<string, string>;
  const template = (dictionary as Record<string, string>)[key] ?? fallback[key] ?? key;

  if (!params) {
    return template;
  }

  return template.replace(PARAM_REGEX, (_, token: string) => String(params[token] ?? ''));
};

export const statusLabel = (language: UiLanguage, status: string): string => {
  return t(language, `status_${status}`);
};

export const signalLabel = (language: UiLanguage, signal: string): string => {
  return t(language, `signal_${signal}`);
};

export const roleLabel = (language: UiLanguage, role: string): string => {
  if (!role) {
    return '';
  }

  const key = `role_${role}`;
  const translated = t(language, key);
  return translated === key ? role : translated;
};

export const eventLabel = (language: UiLanguage, eventType: string): string => {
  const key = `event_${eventType}`;
  const translated = t(language, key);
  return translated === key ? eventType : translated;
};

export const tagLabel = (language: UiLanguage, tagCode: string, fallback: string): string => {
  const key = `tag_${tagCode}`;
  const translated = t(language, key);
  return translated === key ? fallback : translated;
};
