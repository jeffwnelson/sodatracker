export interface Product {
  id: string;
  upc: string;
  name: string;
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
