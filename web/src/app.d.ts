// See https://svelte.dev/docs/kit/types#app.d.ts
declare global {
  namespace App {
    // interface Error {}
    // interface Locals {}
    interface PageData {
      // The current signed-in user, resolved in the root layout's server load
      // and exposed via page data (SSR-safe, per-request). null when signed out.
      user: import('$lib/types').User | null;
    }
    // interface PageState {}
    // interface Platform {}
  }
}

export {};
