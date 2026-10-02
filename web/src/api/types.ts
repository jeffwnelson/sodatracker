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
  /** "#rrggbb", or absent to use the default palette. */
  color?: string;
  /** US fl oz per can; the server defaults it to 12. */
  sizeOz?: number;
  /** Caffeine per can; 0 for caffeine-free or unknown. */
  caffeineMg?: number;
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
  agent: AgentStatus;
}

/** The Pi agent's heartbeat state. lastSeen is null until its first heartbeat
 * since the server started. */
export interface AgentStatus {
  online: boolean;
  lastSeen: string | null;
}

/** A message from GET /events. data always carries machineId. */
export interface LiveEvent {
  type: string;
  timestamp: string;
  data: { machineId?: string } & Record<string, unknown>;
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

export type AnalyticsKind =
  | "consumption"
  | "products"
  | "inventory"
  | "spend"
  | "highlights"
  | "uptime";
export type Bucket = "day" | "week" | "month";

export interface AnalyticsRange {
  from: string;
  to: string;
  days: number;
  bucket: Bucket;
  timezone: string;
  /** First dispense or restock ever; null with neither. */
  trackingSince: string | null;
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
  usedShare: number | null;
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
    usedShare: number | null;
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
  highlights: HighlightsAnalytics;
  uptime: UptimeAnalytics;
}

export interface TimeOfDay {
  /** Minutes after local midnight. */
  avgMinute: number;
  /** The earliest first can, or the latest last can. */
  record: string;
}

export interface CountAt {
  at: string;
  count: number;
}

export interface HighlightsAnalytics {
  range: AnalyticsRange;
  firstCan: TimeOfDay | null;
  lastCan: TimeOfDay | null;
  busiestDay: CountAt | null;
  busiestHour: CountAt | null;
  /** Days by the product of their first can. */
  firstCanProducts: ProductCount[];
  /** Days by the product of their last can. */
  lastCanProducts: ProductCount[];
  /** Cans dispensed 11:00–14:00 local time, by product. */
  lunchProducts: ProductCount[];
  ounces: number;
  caffeineMg: number;
  aluminumLb: number;
}

export interface UptimeAnalytics {
  range: AnalyticsRange;
  trackingSince: string | null;
  onlineHours: number;
  offlineHours: number;
  untrackedHours: number;
  uptime: number | null;
  outages: number;
  longestOutageHours: number;
  series: {
    bucket: string;
    onlineHours: number;
    offlineHours: number;
    untrackedHours: number;
  }[];
  outageList: { from: string; to: string; hours: number; ongoing: boolean }[];
}

export interface BackupFile {
  name: string;
  size: number;
  createdAt: string;
}

/** GET /admin/backup. The token itself is never returned, only tokenSet. */
export interface BackupStatus {
  pushEnabled: boolean;
  repo: string;
  tokenSet: boolean;
  /** Hours between scheduled snapshots; 0 means off. */
  intervalHours: number;
  keep: number;
  lastPushAt: string | null;
  lastPushedFile?: string;
  lastError?: string;
  lastErrorAt?: string;
  /** The last snapshot or push failed, or the last push is over 36h old. */
  stale: boolean;
  /** Local snapshots, newest first. */
  snapshots: BackupFile[];
}

/** PUT /admin/backup/config. Omit token to keep it; "" clears it. */
export interface BackupUpdate {
  repo: string;
  token?: string;
  intervalHours: number;
  keep: number;
}


export interface ProductCount {
  productId: string;
  name: string;
  count: number;
}

/** GET /admin/slack. The webhook and token are never returned. */
export interface SlackStatus {
  webhookSet: boolean;
  appTokenSet: boolean;
  /** The /soda Socket Mode connection. */
  commands: "off" | "connecting" | "connected" | "error";
  commandsError?: string;
}

/** PUT /admin/slack. Omit a field to keep it; "" clears it. */
export interface SlackUpdate {
  webhookUrl?: string;
  appToken?: string;
}
