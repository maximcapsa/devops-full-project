import { listProducts, type Product } from "@/lib/api";
import ProductCard from "@/components/ProductCard";

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
      <h1 className="mb-6 text-3xl font-bold">Products</h1>

      {error && (
        <p className="mb-6 rounded bg-red-100 p-4 text-red-700">
          Could not load products: {error}
        </p>
      )}

      <ul className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {products.map((p) => (
          <ProductCard key={p.id} product={p} />
        ))}
      </ul>

      {!error && products.length === 0 && (
        <p className="text-gray-500">No products yet.</p>
      )}
    </main>
  );
}
