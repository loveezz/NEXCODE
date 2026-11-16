import { create } from "zustand";
type State = {
  code: string;
  exerciseId: number | null;
  setCode: (code: string) => void;
  initialize: (id: number, code: string) => void;
};
export const useEditorStore = create<State>((set) => ({
  code: "",
  exerciseId: null,
  setCode: (code) => set({ code }),
  initialize: (exerciseId, code) => set({ exerciseId, code }),
}));
