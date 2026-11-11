import ProofCheck.SierraEntriesFieldValid

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

def SierraEntriesFieldError (result : GoInterface) (pos : w64) : SierraEntriesFieldFailure → IProp GF
  | .entries reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      SierraEntryValuesFailureOwned err pos reason)

theorem sierraEntriesFieldError_nonNil (result : GoInterface) (pos : w64) (reason : SierraEntriesFieldFailure) :
    SierraEntriesFieldError (GF := GF) result pos reason ⊢ ⌜result ≠ interface.nil⌝ := by
  cases reason <;> isimp only [SierraEntriesFieldError]
  all_goals
    iintro ⟨%err, %Heq, _⟩
    ipureintro; rw [Heq]; simp

/-- Failed calls retain every old field while exposing exact cursor movement,
    entered depth, and the owned diagnostic from the actual helper. -/
theorem wp_sierraEntries_field_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : SierraEntriesValue) (field : SierraEntriesField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat) (reason : SierraEntriesFieldFailure)
    (failure : SierraEntriesFieldRejected d field wire consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SierraEntriesOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (sierraEntriesFieldReader field))) (Val #c)) (Val #(structFieldRef SierraEntryPointsByType (sierraEntriesFieldName field) dst)))
    {{ (result : GoInterface), RET #result; SierraEntriesOwned dst old ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ SierraEntriesFieldError result (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bounds := sierraEntriesField_rejected_bounds _ _ _ _ _ _ _ failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have progress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by
    constructor; · word
    constructor; · word
    constructor <;> word
  cases failure with
  | @constructors wire consumed offset extraDepth why failure =>
    icases sierraEntries_constructors_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [sierraEntriesFieldReader, sierraEntriesFieldName]
    wp_apply wp_readSierraEntryValues_rejected validator c (structFieldRef SierraEntryPointsByType go!"Constructor" dst) slice
      v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %slice %old.constructors [Hfield Hprevious]
    · iframe Hfield Hprevious
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold SierraEntriesFieldError
      iexists err
      iframe Herr
      ipureintro; rfl
    ipureintro; exact progress
  | @external wire consumed offset extraDepth why failure =>
    icases sierraEntries_external_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [sierraEntriesFieldReader, sierraEntriesFieldName]
    wp_apply wp_readSierraEntryValues_rejected validator c (structFieldRef SierraEntryPointsByType go!"External" dst) slice
      v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %slice %old.external [Hfield Hprevious]
    · iframe Hfield Hprevious
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold SierraEntriesFieldError
      iexists err
      iframe Herr
      ipureintro; rfl
    ipureintro; exact progress
  | @l1Handler wire consumed offset extraDepth why failure =>
    icases sierraEntries_l1Handler_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [sierraEntriesFieldReader, sierraEntriesFieldName]
    wp_apply wp_readSierraEntryValues_rejected validator c (structFieldRef SierraEntryPointsByType go!"L1Handler" dst) slice
      v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %slice %old.l1Handler [Hfield Hprevious]
    · iframe Hfield Hprevious
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold SierraEntriesFieldError
      iexists err
      iframe Herr
      ipureintro; rfl
    ipureintro; exact progress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesFieldError_nonNil
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_sierraEntries_field_rejected
