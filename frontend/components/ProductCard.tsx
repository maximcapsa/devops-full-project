"use client";

import { formatPrice, type Product } from "@/lib/api";
import { useCart } from "@/lib/cart";

export default function ProductCard({ product }: { product: Product }) {
  const { add } = useCart();
  return (
    <li className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
      <h2 className="text-lg font-semibold">{product.name}</h2>
      <p className="mt-1 text-sm text-gray-600">{product.description}</p>
      <div className="mt-3 flex items-center justify-between">
        <p className="font-mono text-gray-900">{formatPrice(product.priceCents)}</p>
        <button
          onClick={() => add(product)}
          className="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-500"
        >
          Add to cart
        </button>
      </div>
    </li>
  );
}
