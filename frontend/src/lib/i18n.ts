import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import en from '../locales/en.json'
import zh from '../locales/zh.json'

// Language is stored in localStorage (manual switch) or detected from the
// browser; the toggle in the layout updates both.
const stored = localStorage.getItem('gourl-lang')
const detected = navigator.language?.toLowerCase().startsWith('zh') ? 'zh' : 'en'

void i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    zh: { translation: zh },
  },
  lng: stored ?? detected,
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
})

export function setLanguage(lang: 'en' | 'zh') {
  localStorage.setItem('gourl-lang', lang)
  void i18n.changeLanguage(lang)
}

export default i18n
