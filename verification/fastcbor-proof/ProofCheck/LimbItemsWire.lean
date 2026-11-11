import ProofCheck.LimbWideStep

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

structure LimbValueWire (depth : Nat) where
  bytes : List w8
  value : GoArray w64 4
  valid : FeltValid depth bytes (some value)

/-- Sequence witnesses retain arbitrary felt encodings, including short and
    extended arrays. They do not restrict the independent sequence relation. -/
theorem limbItems_witness (depth count : Nat) (wire : List w8) (values : List (GoArray w64 4))
    (valid : LimbItemsValid depth count wire values) :
    ∃ items : List (LimbValueWire depth), items.length = count ∧
      items.flatMap LimbValueWire.bytes = wire ∧ items.map LimbValueWire.value = values := by
  induction valid with
  | nil => exact ⟨[], rfl, rfl, rfl⟩
  | @cons n item tail value values head rest ih =>
    obtain ⟨items, count, bytes, vals⟩ := ih
    refine ⟨⟨item, value, head⟩ :: items, ?_, ?_, ?_⟩ <;> simp [count, bytes, vals]

/-- Replacing the next output slot extends exactly the initialized prefix. -/
theorem limbValues_prefix_step (done tail : List (GoArray w64 4)) (old value : GoArray w64 4) :
    (done ++ old :: tail).set done.length value = (done ++ [value]) ++ tail := by
  rw [List.set_append_right _ _ (Nat.le_refl _)]
  simp

/-- Taking another suffix of the local input window preserves its relationship
    to the original slice, including its backing pointer and capacity. -/
theorem limbWindow_advance [GoLocalContext] [GoSemanticsFunctions] [go.ArraySemantics] (data : GoSlice) (off step : w64)
    (hb : 0 ≤ sint.Z off + sint.Z step ∧ sint.Z off + sint.Z step < 2^63) :
    slice.slice (slice.slice data w8 off data.len) w8 step (data.len - off) =
      slice.slice data w8 (off + step) data.len := by
  simp only [slice.slice, sliceIndexRef]
  rw [← go.arrayIndexRef_add]
  have hs : sint.Z (off + step) = sint.Z off + sint.Z step := by word
  rw [hs]
  congr 1 <;> word

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.limbItems_witness
#print axioms Perennial.github_com.NethermindEth.juno.core.limbValues_prefix_step
#print axioms Perennial.github_com.NethermindEth.juno.core.limbWindow_advance
