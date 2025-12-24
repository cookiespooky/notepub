declare module "react-dom" {
  /**
   * Minimal type shim for React server actions form state hook used in Next.js app router.
   * Matches the runtime shape used in AccountForms.
   */
  export function useFormState<S, P>(
    action: (state: S, payload: P) => S | Promise<S>,
    initialState: S,
    permalink?: string,
  ): [Awaited<S>, (payload: P) => void];

  /**
   * Minimal type shim for form status hook used with server actions.
   */
  export function useFormStatus(): { pending: boolean };
}
