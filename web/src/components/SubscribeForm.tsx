"use client";

import { useState, type FormEvent } from "react";
import {
  ApiError,
  createSubscription,
  type CreateSubscriptionInput,
} from "@/lib/api-client";
import { isValidEmail, isValidPlz, isValidRadius } from "@/lib/format";
import type { Messages } from "@/i18n";

export interface SubscribeFormProps {
  messages: Messages;
  /** Prefill from the active filter, if any. */
  initialPlz?: string;
  initialRadius?: number;
}

type Status = "idle" | "submitting" | "success" | "error";

interface FieldErrors {
  email?: string;
  plz?: string;
  radiusKm?: string;
}

export function SubscribeForm({
  messages,
  initialPlz = "",
  initialRadius = 50,
}: SubscribeFormProps) {
  const t = messages.subscribe;

  const [email, setEmail] = useState("");
  const [plz, setPlz] = useState(initialPlz);
  const [radius, setRadius] = useState(initialRadius);
  const [status, setStatus] = useState<Status>("idle");
  const [errors, setErrors] = useState<FieldErrors>({});
  const [serverError, setServerError] = useState<string | null>(null);

  function validate(): FieldErrors {
    const next: FieldErrors = {};
    if (!isValidEmail(email)) next.email = t.emailInvalid;
    if (!isValidPlz(plz)) next.plz = t.plzInvalid;
    if (!isValidRadius(radius)) next.radiusKm = t.radiusInvalid;
    return next;
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setServerError(null);

    const validation = validate();
    setErrors(validation);
    if (Object.keys(validation).length > 0) {
      setStatus("error");
      return;
    }

    const input: CreateSubscriptionInput = {
      email: email.trim(),
      plz: plz.trim(),
      radiusKm: radius,
      productId: null,
    };

    setStatus("submitting");
    try {
      await createSubscription(input);
      setStatus("success");
    } catch (err) {
      if (err instanceof ApiError) {
        // Surface server-side field validation.
        if (err.field === "email") {
          setErrors((prev) => ({ ...prev, email: err.message }));
        } else if (err.field === "plz") {
          setErrors((prev) => ({ ...prev, plz: err.message }));
        } else if (err.field === "radiusKm") {
          setErrors((prev) => ({ ...prev, radiusKm: err.message }));
        } else {
          setServerError(err.message || t.genericError);
        }
      } else {
        setServerError(t.genericError);
      }
      setStatus("error");
    }
  }

  if (status === "success") {
    return (
      <section className="rounded-lg border border-green-200 bg-green-50 p-4 shadow-sm">
        <h2 className="text-base font-semibold text-green-900">{t.success}</h2>
        <p className="mt-1 text-sm text-green-800">{t.successDetail}</p>
      </section>
    );
  }

  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <h2 className="text-base font-semibold text-slate-900">{t.heading}</h2>
      <p className="mb-3 text-sm text-slate-500">{t.description}</p>

      <form onSubmit={handleSubmit} noValidate className="space-y-3">
        <div>
          <label
            htmlFor="subscribe-email"
            className="mb-1 block text-sm font-medium text-slate-700"
          >
            {t.emailLabel}
          </label>
          <input
            id="subscribe-email"
            name="email"
            type="email"
            autoComplete="email"
            placeholder={t.emailPlaceholder}
            value={email}
            aria-invalid={Boolean(errors.email)}
            aria-describedby={errors.email ? "subscribe-email-error" : undefined}
            onChange={(e) => setEmail(e.target.value)}
            className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500"
          />
          {errors.email && (
            <p id="subscribe-email-error" role="alert" className="mt-1 text-sm text-red-600">
              {errors.email}
            </p>
          )}
        </div>

        <div className="flex gap-3">
          <div className="flex-1">
            <label
              htmlFor="subscribe-plz"
              className="mb-1 block text-sm font-medium text-slate-700"
            >
              {t.plzLabel}
            </label>
            <input
              id="subscribe-plz"
              name="plz"
              type="text"
              inputMode="numeric"
              autoComplete="postal-code"
              maxLength={5}
              placeholder={t.plzPlaceholder}
              value={plz}
              aria-invalid={Boolean(errors.plz)}
              aria-describedby={errors.plz ? "subscribe-plz-error" : undefined}
              onChange={(e) => setPlz(e.target.value.replace(/[^\d]/g, ""))}
              className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500"
            />
            {errors.plz && (
              <p id="subscribe-plz-error" role="alert" className="mt-1 text-sm text-red-600">
                {errors.plz}
              </p>
            )}
          </div>

          <div className="w-28">
            <label
              htmlFor="subscribe-radius"
              className="mb-1 block text-sm font-medium text-slate-700"
            >
              {t.radiusLabel}
            </label>
            <input
              id="subscribe-radius"
              name="radiusKm"
              type="number"
              min={1}
              max={500}
              value={radius}
              aria-invalid={Boolean(errors.radiusKm)}
              aria-describedby={errors.radiusKm ? "subscribe-radius-error" : undefined}
              onChange={(e) => setRadius(Number(e.target.value))}
              className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500"
            />
            {errors.radiusKm && (
              <p id="subscribe-radius-error" role="alert" className="mt-1 text-sm text-red-600">
                {errors.radiusKm}
              </p>
            )}
          </div>
        </div>

        {serverError && (
          <p role="alert" className="text-sm text-red-600">
            {serverError}
          </p>
        )}

        <button
          type="submit"
          disabled={status === "submitting"}
          className="w-full rounded-md bg-sky-600 px-4 py-2 text-sm font-medium text-white hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {status === "submitting" ? t.submitting : t.submit}
        </button>
      </form>
    </section>
  );
}

export default SubscribeForm;
