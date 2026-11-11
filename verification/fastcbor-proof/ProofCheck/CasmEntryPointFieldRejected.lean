import ProofCheck.CasmEntryPointFieldValid

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

def CasmEntryPointFieldError (result : GoInterface) (pos : w64) : CasmEntryPointFieldFailure → IProp GF
  | .felt reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ FeltFailureError err pos reason (feltFailureKind reason kinds))
  | .strings reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      StringsFailureOwned err pos reason)
  | .scalar reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ ExpectedFailureError err pos (W8 0) reason (expectedFailureKind reason kinds))

theorem casmEntryPointFieldError_nonNil (result : GoInterface) (pos : w64) (reason : CasmEntryPointFieldFailure) :
    CasmEntryPointFieldError (GF := GF) result pos reason ⊢ ⌜result ≠ interface.nil⌝ := by
  cases reason <;> isimp only [CasmEntryPointFieldError]
  all_goals
    iintro ⟨%err, %Heq, _⟩
    ipureintro; rw [Heq]; simp

/-- Failed calls retain every old field while exposing exact cursor movement,
    entered depth, and the owned diagnostic from the actual helper. -/
theorem wp_casmEntryPoint_field_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : CasmEntryPointValue) (field : CasmEntryPointField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat) (reason : CasmEntryPointFieldFailure)
    (failure : CasmEntryPointFieldRejected d field wire consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       CasmEntryPointOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (casmEntryPointFieldReader field))) (Val #c)) (Val #(structFieldRef CasmEntryPoint (casmEntryPointFieldName field) dst)))
    {{ (result : GoInterface), RET #result; CasmEntryPointOwned dst old ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ CasmEntryPointFieldError result (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bounds := casmEntryPointField_rejected_bounds _ _ _ _ _ _ _ failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have progress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by
    constructor; · word
    constructor; · word
    constructor <;> word
  cases failure with
  | @offset wire why failure =>
    icases casmEntryPoint_offset_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    simp only [casmEntryPointFieldReader, casmEntryPointFieldName]
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 0) why) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    have hread := wp_readScalar_failure (GF := GF) false c (structFieldRef CasmEntryPoint go!"Offset" dst) old.offset
      v xs wire dq why (expectedFailureKind why kinds) failure hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hkind Hfield Hc Hs
    ihave Howned := Hclose $$ %old.offset Hfield
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmEntryPointFieldError
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress
  | @builtins wire offset extraDepth why failure =>
    icases casmEntryPoint_builtins_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [casmEntryPointFieldReader, casmEntryPointFieldName]
    wp_apply wp_readStrings_rejected validator c (structFieldRef CasmEntryPoint go!"Builtins" dst) slice
      v xs wire dq d _ extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %slice %old.builtins [Hfield Hprevious]
    · iframe Hfield Hprevious
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmEntryPointFieldError
      iexists err
      iframe Herr
      ipureintro; rfl
    ipureintro; exact progress
  | @selector wire offset why failure =>
    icases casmEntryPoint_selector_acc dst old $$ Howned with ⟨%ptr, Hfield, Hprevious, Hclose⟩
    simp only [casmEntryPointFieldReader, casmEntryPointFieldName]
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c (structFieldRef CasmEntryPoint go!"Selector" dst) ptr
      v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hkind Hfield Hc Hs
    ihave Howned := Hclose $$ %ptr %old.selector [Hfield Hprevious]
    · iframe Hfield Hprevious
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmEntryPointFieldError
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointFieldError_nonNil
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_casmEntryPoint_field_rejected
