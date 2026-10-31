/** The API picocrank's Dialog component exposes through a template ref. */
export interface DialogApi {
  /** Open the dialog (showModal(), or show() when non-modal). */
  show: () => void
  /** Close the dialog, optionally with a return value. */
  close: (returnValue?: string) => void
  /** The underlying <dialog> element. */
  dialog: HTMLDialogElement | null
}
