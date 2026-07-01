"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { formatPrice, placeOrder } from "@/lib/api";
import { useCart } from "@/lib/cart";

export default function CartPage() {
  const { lines, totalCents, setQuantity, remove, clear } = useCart();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const router = useRouter();

  async function checkout() {
    setSubmitting(true);
    setError(null);
    try {
      const order = await placeOrder(
        lines.map((l) => ({ productId: l.product.id, quantity: l.quantity }))
      );
      clear();
      router.push(`/orders/${order.id}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to place order");
      setSubmitting(false);
    }
  }

  if (lines.length === 0) {
    return (
      <main className="mx-auto max-w-4xl p-8">
        <h1 className="mb-6 text-3xl font-bold">Cart</h1>
        <p className="text-gray-500">
          Your cart is empty.{" "}
          <Link href="/" className="text-blue-600 underline">
            Browse products
          </Link>
        </p>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-4xl p-8">
      <h1 className="mb-6 text-3xl font-bold">Cart</h1>

      <ul className="divide-y divide-gray-200 rounded-lg border border-gray-200 bg-white">
        {lines.map((l) => (
          <li key={l.product.id} className="flex items-center justify-between gap-4 p-4">
            <div className="min-w-0">
              <p className="font-semibold">{l.product.name}</p>
              <p className="text-sm text-gray-600">{formatPrice(l.product.priceCents)} each</p>
            </div>
            <div className="flex items-center gap-3">
              <div className="flex items-center rounded border border-gray-300">
                <button
                  aria-label={`decrease ${l.product.name}`}
                  onClick={() => setQuantity(l.product.id, l.quantity - 1)}
                  className="px-3 py-1 hover:bg-gray-100"
                >
                  −
                </button>
                <span className="w-10 text-center font-mono">{l.quantity}</span>
                <button
                  aria-label={`increase ${l.product.name}`}
                  onClick={() => setQuantity(l.product.id, l.quantity + 1)}
                  className="px-3 py-1 hover:bg-gray-100"
                >
                  +
                </button>
              </div>
              <p className="w-24 text-right font-mono">
                {formatPrice(parseInt(l.product.priceCents, 10) * l.quantity)}
              </p>
              <button
                onClick={() => remove(l.product.id)}
                className="text-sm text-red-600 hover:underline"
              >
                Remove
              </button>
            </div>
          </li>
        ))}
      </ul>

      <div className="mt-6 flex items-center justify-between">
        <p className="text-xl font-bold">
          Total: <span className="font-mono">{formatPrice(totalCents)}</span>
        </p>
        <button
          onClick={checkout}
          disabled={submitting}
          className="rounded-lg bg-blue-600 px-6 py-3 font-medium text-white hover:bg-blue-500 disabled:opacity-50"
        >
          {submitting ? "Placing order…" : "Place order"}
        </button>
      </div>

      {error && <p className="mt-4 rounded bg-red-100 p-4 text-red-700">{error}</p>}
    </main>
  );
}
