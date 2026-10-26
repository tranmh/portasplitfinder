export interface StockBadgeProps {
  inStock: boolean;
  inStockLabel: string;
  outOfStockLabel: string;
}

export function StockBadge({ inStock, inStockLabel, outOfStockLabel }: StockBadgeProps) {
  return (
    <span
      className={
        inStock
          ? "inline-flex items-center rounded-full bg-emerald-600 px-2 py-0.5 text-xs font-semibold text-white"
          : "inline-flex items-center rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600"
      }
    >
      {inStock ? inStockLabel : outOfStockLabel}
    </span>
  );
}

export default StockBadge;
