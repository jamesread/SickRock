import type { InjectionKey } from 'vue'

export type AccordionContext = {
  isOpen: (id: string) => boolean
  toggle: (id: string) => void
}

export const accordionContextKey: InjectionKey<AccordionContext> = Symbol('accordion')
