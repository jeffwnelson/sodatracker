export interface HealthResponse {
  status: string;
  version: string;
}

export interface Product {
  id: string;
  upc?: string;
  name: string;
  description?: string;
  costPerCan?: number;
  imageUrl?: string;
}

export interface Machine {
  id: string;
  name: string;
  location: string;
  slotCount: number;
}

export interface Slot {
  machineId: string;
  position: number;
  productId?: string;
  gpioPin?: number;
  capacity: number;
  lowThreshold: number;
}

export interface SlotStatus {
  slot: Slot;
  product?: Product;
  currentStock: number;
  status: "ok" | "low" | "empty";
}

export interface MachineStatus {
  machine: Machine;
  slots: SlotStatus[];
}

export interface Dispense {
  id: string;
  machineId: string;
  productId: string;
  slotPosition: number;
  sourceType: string;
  timestamp: string;
}

export interface RestockSlot {
  position: number;
  quantity: number;
}

export interface Restock {
  id: string;
  machineId: string;
  userId?: string;
  slots: RestockSlot[];
  timestamp: string;
}

export interface User {
  id: string;
  username: string;
  role: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface TimeSeriesPoint {
  hour: string;
  count: number;
}

export interface ProductRanking {
  productId: string;
  productName: string;
  count: number;
  totalCost: number;
}

export interface DispenseStats {
  totalCount: number;
  totalCost: number;
  timeSeries: TimeSeriesPoint[];
  topProducts: ProductRanking[];
}

export interface AlertConfig {
  id: string;
  machineId: string;
  slotPosition: number;
  lowThreshold: number;
  enabled: boolean;
}

export interface AlertEvent {
  id: string;
  machineId: string;
  slotPosition: number;
  productId: string;
  alertType: "low_stock" | "out_of_stock";
  currentStock: number;
  threshold: number;
  cleared: boolean;
  timestamp: string;
  clearedAt?: string;
}

export interface StockPrediction {
  machineId: string;
  slotPosition: number;
  productName?: string;
  currentStock: number;
  dispensesPerDay: number;
  estimatedDaysLeft: number;
  predictedStockout?: string;
}

// --- Analytics ---

export type AnalyticsKind = "consumption" | "products" | "inventory" | "spend";
export type Bucket = "day" | "week" | "month";

export interface AnalyticsRange {
  from: string;
  to: string;
  days: number;
  bucket: Bucket;
  timezone: string;
}

export interface ConsumptionAnalytics {
  range: AnalyticsRange;
  total: number;
  previousTotal: number | null;
  perDay: number;
  weekdayPerDay: number;
  weekendPerDay: number;
  peakHour: number;
  peakWeekday: number;
  series: { bucket: string; count: number; trend: number }[];
  /** [weekday Monday=0][local hour] */
  heatmap: number[][];
}

export interface ProductStat {
  productId: string;
  name: string;
  count: number;
  share: number;
  cost: number;
  rank: number;
  previousRank: number | null;
  previousCount: number | null;
  slots: number[];
  lastDispensed: string | null;
}

export interface DaypartStat {
  name: string;
  startHour: number;
  endHour: number;
  total: number;
  counts: Record<string, number>;
}

export interface ProductAnalytics {
  range: AnalyticsRange;
  products: ProductStat[];
  series: { bucket: string; counts: Record<string, number> }[];
  dayparts: DaypartStat[];
}

export interface SlotInventory {
  position: number;
  productId?: string;
  productName?: string;
  capacity: number;
  lowThreshold: number;
  currentStock: number | null;
  stockoutHours: number;
  stockouts: number;
  restocks: number;
  cansRestocked: number;
  dispensed: number;
  avgRestockIntervalDays: number | null;
  sellThrough: number | null;
  untrackedDispenses: number;
}

export interface InventoryAnalytics {
  range: AnalyticsRange;
  summary: {
    stockoutHours: number;
    stockouts: number;
    restockCount: number;
    cansRestocked: number;
    avgRestockIntervalDays: number | null;
    sellThrough: number | null;
    untrackedDispenses: number;
  };
  slots: SlotInventory[];
  series: { bucket: string; levels: Record<string, number | null> }[];
  restocks: {
    id: string;
    timestamp: string;
    cans: number;
    slots: number;
    daysSincePrevious: number | null;
  }[];
}

export interface SpendAnalytics {
  range: AnalyticsRange;
  total: number;
  previousTotal: number | null;
  count: number;
  perDay: number;
  projectedMonthly: number;
  avgCostPerCan: number;
  restockedValue: number;
  unpricedDispenses: number;
  series: { bucket: string; spend: number; count: number; cumulative: number }[];
  byProduct: {
    productId: string;
    name: string;
    spend: number;
    count: number;
    share: number;
  }[];
}

export interface AnalyticsByKind {
  consumption: ConsumptionAnalytics;
  products: ProductAnalytics;
  inventory: InventoryAnalytics;
  spend: SpendAnalytics;
}
