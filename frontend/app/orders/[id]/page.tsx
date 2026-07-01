"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  formatPrice,
  getNotifications,
  getOrder,
  type Notification,
  type Order,
} from "@/lib/api";

const TERMINAL = new Set(["PAID", "PAYMENT_FAILED", "REJECTED"]);
const POLL_MS = 2000;

const STATUS_STYLE: Record<string, string> = {
  PENDING: "bg-yellow-100 text-yellow-800",
  PAID: "bg-green-100 text-green-800",
  PAYMENT_FAILED: "bg-red-100 text-red-800",
  REJECTED: "bg-red-100 text-red-800",
};

export default function OrderPage() {
  const { id } = useParams<{ id: string }>();
  const [order, setOrder] = useState<Order | null>(null);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;

    async function poll() {
      try {
        const [o, n] = await Promise.all([getOrder(id), getNotifications(id)]);
        if (stopped) return;
        setOrder(o);
        setNotifications(n);
        setError(null);
        if (TERMINAL.has(o.status)) return; // saga settled — stop polling
      } catch (e) {
        if (stopped) return;
        setError(e instanceof Error ? e.message : "failed to load order");
      }
      timer = setTimeout(poll, POLL_MS);
    }

    poll();
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [id]);

  return (
    <main className="mx-auto max-w-4xl p-8">
      <h1 className="mb-2 text-3xl font-bold">Order status</h1>
      <p className="mb-6 font-mono text-sm text-gray-500">{id}</p>

      {error && <p className="mb-6 rounded bg-red-100 p-4 text-red-700">{error}</p>}

      {order && (
        <div className="mb-8 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
          <div className="flex items-center justify-between">
            <span
              className={`rounded-full px-3 py-1 text-sm font-semibold ${
                STATUS_STYLE[order.status] ?? "bg-gray-100 text-gray-800"
              }`}
            >
              {order.status}
            </span>
            <p className="text-xl font-bold">
              <span className="font-mono">{formatPrice(order.totalCents)}</span>
            </p>
          </div>
          {!TERMINAL.has(order.status) && (
            <p className="mt-3 text-sm text-gray-500">Watching for updates…</p>
          )}
        </div>
      )}

      <h2 className="mb-3 text-xl font-semibold">Timeline</h2>
      {notifications.length === 0 ? (
        <p className="text-gray-500">No events yet.</p>
      ) : (
        <ol className="relative ml-3 border-l border-gray-300">
          {notifications.map((n) => (
            <li key={n.id} className="mb-6 ml-6">
              <span className="absolute -left-1.5 mt-1.5 h-3 w-3 rounded-full bg-blue-600" />
              <p className="font-semibold">{n.type}</p>
              <p className="text-sm text-gray-600">{n.message}</p>
              <p className="mt-1 text-xs text-gray-400">
                {new Date(n.createdAt).toLocaleTimeString()}
              </p>
            </li>
          ))}
        </ol>
      )}

      <Link href="/" className="mt-8 inline-block text-blue-600 underline">
        Continue shopping
      </Link>
    </main>
  );
}
