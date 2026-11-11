import ProofCheck.FeltReserveGrow

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- A reserve contains only unused zeroed slots and exposes its whole capacity. -/
def FeltReserve (data : GoSlice) (count : Nat) : IProp GF := iprop(
  data ↦* List.replicate count (zero_val (GoArray w64 4)) ∗ ⌜data.len = data.cap⌝)

/-- Exact count transition: reuse consumes one slot; refill supplies at least
    want slots before consuming one. Capacity rounding remains unspecified. -/
def FeltReserveStep (before after : Nat) (want : w64) : Prop :=
  if before = 0 then sint.Z want ≤ (after : Int) + 1 else after + 1 = before

theorem feltReserve_nil : ⊢ (FeltReserve slice.nil 0 : IProp GF) := by
  unfold FeltReserve
  simp only [List.replicate_zero]
  isplit
  · iapply ownSlice_nil
  · ipureintro; rfl

/-- Complete actual allocator contract for every owned zeroed reserve and
    positive request. The returned slot and remaining reserve are disjoint. -/
theorem wp_nextFelt_complete (spare : Loc) (data : GoSlice) (count : Nat) (want : w64)
    (positive : 0 < sint.Z want) :
    {{ spare ↦ data ∗ FeltReserve (GF := GF) data count }}
      (App (App (Val (@! cborNextFelt)) (Val #spare)) (Val #want))
    {{ (out : Loc), RET #out; ∃ (remaining : GoSlice) (after : Nat),
       out ↦ (zero_val (GoArray w64 4)) ∗ spare ↦ remaining ∗ FeltReserve remaining after ∗
       ⌜FeltReserveStep count after want⌝ }} := by
  wp_start_folded as ⟨Hspare, Hreserve⟩
  unfold FeltReserve
  icases Hreserve with ⟨Hdata, %Hcap⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hdata
  simp only [List.length_replicate] at Hlen
  by_cases empty : count = 0
  · have hempty : data.len = W64 0 := by word
    wp_apply wp_nextFelt_grow spare data want hempty positive $$ [Hspare]
      as %grown ⟨Hout, Hspare, Htail, %Hgrown⟩
    · iexact Hspare
    iapply HΦ $$ %grown.ptr
    iexists (slice.slice grown (GoArray w64 4) (W64 1) grown.len), (sint.nat grown.len - 1)
    iframe Hout Hspare Htail
    ipureintro
    constructor
    · simp only [slice.slice]; rw [Hgrown.1]
    · unfold FeltReserveStep
      rw [if_pos empty]
      word
  · have hcount : count = (count - 1) + 1 := by omega
    have hzeros : List.replicate count (zero_val (GoArray w64 4)) =
        zero_val (GoArray w64 4) :: List.replicate (count - 1) (zero_val (GoArray w64 4)) := by
      simpa only [List.replicate_succ] using
        congrArg (fun n => List.replicate n (zero_val (GoArray w64 4))) hcount
    isimp only [hzeros] at Hdata
    wp_apply wp_nextFelt_reuse spare data (zero_val (GoArray w64 4))
      (List.replicate (count - 1) (zero_val (GoArray w64 4))) want
      $$ [Hspare Hdata] with ⟨Hout, Hspare, Htail⟩
    · iframe Hspare Hdata
    iapply HΦ $$ %data.ptr
    iexists (slice.slice data (GoArray w64 4) (W64 1) data.len), (count - 1)
    iframe Hout Hspare Htail
    ipureintro
    constructor
    · simp only [slice.slice]; rw [Hcap]
    · unfold FeltReserveStep
      rw [if_neg empty]
      omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltReserve_nil
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextFelt_complete
