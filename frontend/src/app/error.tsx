"use client";
import { ErrorState } from "@/components/common";
export default function Error({
  error,
  reset,
}: {
  error: Error;
  reset: () => void;
}) {
  return (
    <div className="standalone">
      <ErrorState error={error} retry={reset} />
    </div>
  );
}
