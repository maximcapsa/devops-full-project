// API client for the bff REST endpoints.
//
// Server components (running inside the container) reach the bff over the
// compose/cluster network via BFF_INTERNAL_URL. Browser-side code uses the
// public NEXT_PUBLIC_BFF_URL. Product listing is fetched server-side, so it
// uses the internal URL.

export type Product = {
  id: string;
  name: string;
  description: string;
  priceCents: string; // proto int64 is serialized as a JSON string
  stock: number;
};

const BFF_INTERNAL_URL = process.env.BFF_INTERNAL_URL ?? "http://localhost:8080";

export async function listProducts(): Promise<Product[]> {
  const res = await fetch(`${BFF_INTERNAL_URL}/v1/products`, { cache: "no-store" });
  if (!res.ok) {
    throw new Error(`bff returned ${res.status}`);
  }
  const data = (await res.json()) as { products?: Product[] };
  return data.products ?? [];
}

export function formatPrice(cents: string | number): string {
  const n = typeof cents === "string" ? parseInt(cents, 10) : cents;
  return (n / 100).toLocaleString("en-US", { style: "currency", currency: "USD" });
}
