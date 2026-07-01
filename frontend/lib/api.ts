// API client for the bff REST endpoints.
//
// Server components (running inside the container) reach the bff over the
// compose/cluster network via BFF_INTERNAL_URL. Browser-side code uses the
// public NEXT_PUBLIC_BFF_URL (baked in at build time).

export type Product = {
  id: string;
  name: string;
  description: string;
  priceCents: string; // proto int64 is serialized as a JSON string
  stock: number;
};

export type OrderItem = { productId: string; quantity: number };

export type Order = {
  id: string;
  status: string;
  items: OrderItem[];
  totalCents: string;
  createdAt: string;
};

export type Notification = {
  id: string;
  orderId: string;
  type: string;
  message: string;
  createdAt: string;
};

const BFF_INTERNAL_URL = process.env.BFF_INTERNAL_URL ?? "http://localhost:8080";
const BFF_PUBLIC_URL = process.env.NEXT_PUBLIC_BFF_URL ?? "http://localhost:8080";

async function ok<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new Error(`bff returned ${res.status}${body ? `: ${body}` : ""}`);
  }
  return res.json() as Promise<T>;
}

// --- server-side ---

export async function listProducts(): Promise<Product[]> {
  const res = await fetch(`${BFF_INTERNAL_URL}/v1/products`, { cache: "no-store" });
  const data = await ok<{ products?: Product[] }>(res);
  return data.products ?? [];
}

// --- browser-side ---

export async function placeOrder(items: OrderItem[]): Promise<Order> {
  const res = await fetch(`${BFF_PUBLIC_URL}/v1/orders`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ items }),
  });
  return ok<Order>(res);
}

export async function getOrder(id: string): Promise<Order> {
  const res = await fetch(`${BFF_PUBLIC_URL}/v1/orders/${id}`, { cache: "no-store" });
  return ok<Order>(res);
}

export async function getNotifications(orderId: string): Promise<Notification[]> {
  const res = await fetch(`${BFF_PUBLIC_URL}/v1/orders/${orderId}/notifications`, {
    cache: "no-store",
  });
  const data = await ok<{ notifications?: Notification[] }>(res);
  return data.notifications ?? [];
}

export function formatPrice(cents: string | number): string {
  const n = typeof cents === "string" ? parseInt(cents, 10) : cents;
  return (n / 100).toLocaleString("en-US", { style: "currency", currency: "USD" });
}
