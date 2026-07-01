import { listProducts, formatPrice, type Product } from "@/lib/api";

// Always render on request so the catalog is fresh (and so `next build` doesn't
// try to reach the bff at build time).
export const dynamic = "force-dynamic";

export default async function Home() {
  let products: Product[] = [];
  let error: string | null = null;
  try {
    products = await listProducts();
  } catch (e) {
    error = e instanceof Error ? e.message : "failed to load products";
  }

  return (
    <main className="mx-auto max-w-4xl p-8">
      <h1 className="mb-6 text-3xl font-bold">Storefront</h1>

      {error && (
        <p className="mb-6 rounded bg-red-100 p-4 text-red-700">
          Could not load products: {error}
        </p>
      )}

      <ul className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {products.map((p) => (
          <li key={p.id} className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
            <h2 className="text-lg font-semibold">{p.name}</h2>
            <p className="mt-1 text-sm text-gray-600">{p.description}</p>
            <p className="mt-3 font-mono text-gray-900">{formatPrice(p.priceCents)}</p>
          </li>
        ))}
      </ul>

      {!error && products.length === 0 && (
        <p className="text-gray-500">No products yet.</p>
      )}
    </main>
  );
}
