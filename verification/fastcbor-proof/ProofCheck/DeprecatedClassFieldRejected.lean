import ProofCheck.DeprecatedClassFieldValid

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

def DeprecatedClassFieldError (result : GoInterface) (pos : w64) : DeprecatedClassFieldFailure → IProp GF
  | .bytes reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ BytesError err pos kinds reason)
  | .entries reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      DeprecatedEntryValuesFailureOwned err pos reason)
  | .string reason => iprop(∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ StringFailureError err pos reason (stringFailureKind reason kinds))

theorem deprecatedClassFieldError_nonNil (result : GoInterface) (pos : w64) (reason : DeprecatedClassFieldFailure) :
    DeprecatedClassFieldError (GF := GF) result pos reason ⊢ ⌜result ≠ interface.nil⌝ := by
  cases reason <;> isimp only [DeprecatedClassFieldError]
  all_goals
    iintro ⟨%err, %Heq, _⟩
    ipureintro; rw [Heq]; simp

/-- Failed calls retain every old field while exposing exact cursor movement,
    entered depth, and the owned diagnostic from the actual helper. -/
theorem wp_deprecatedClass_field_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : DeprecatedClassValue) (field : DeprecatedClassField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat) (reason : DeprecatedClassFieldFailure)
    (failure : DeprecatedClassFieldRejected d field wire consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeprecatedClassOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (deprecatedClassFieldReader field))) (Val #c)) (Val #(structFieldRef DeprecatedCairoClass (deprecatedClassFieldName field) dst)))
    {{ (result : GoInterface), RET #result; DeprecatedClassOwned dst old ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ DeprecatedClassFieldError result (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bounds := deprecatedClassField_rejected_bounds _ _ _ _ _ _ _ failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have progress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by
    constructor; · word
    constructor; · word
    constructor <;> word
  cases failure with
  | @abi wire why failure =>
    icases deprecatedClass_abi_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [deprecatedClassFieldReader, deprecatedClassFieldName]
    wp_apply wp_readRawMessage_rejected c (structFieldRef DeprecatedCairoClass go!"Abi" dst) slice
      v xs wire dq why kinds failure hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hglobals Hfield Hc Hs
    ihave Howned := Hclose $$ %slice %old.abi [Hfield Hprevious]
    · iframe Hfield Hprevious
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold DeprecatedClassFieldError
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress
  | @externals wire consumed offset extraDepth why failure =>
    icases deprecatedClass_externals_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [deprecatedClassFieldReader, deprecatedClassFieldName]
    wp_apply wp_readDeprecatedEntryValues_rejected validator c (structFieldRef DeprecatedCairoClass go!"Externals" dst) slice
      v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %slice %old.externals [Hfield Hprevious]
    · iframe Hfield Hprevious
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold DeprecatedClassFieldError
      iexists err
      iframe Herr
      ipureintro; rfl
    ipureintro; exact progress
  | @l1Handlers wire consumed offset extraDepth why failure =>
    icases deprecatedClass_l1Handlers_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [deprecatedClassFieldReader, deprecatedClassFieldName]
    wp_apply wp_readDeprecatedEntryValues_rejected validator c (structFieldRef DeprecatedCairoClass go!"L1Handlers" dst) slice
      v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %slice %old.l1Handlers [Hfield Hprevious]
    · iframe Hfield Hprevious
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold DeprecatedClassFieldError
      iexists err
      iframe Herr
      ipureintro; rfl
    ipureintro; exact progress
  | @constructors wire consumed offset extraDepth why failure =>
    icases deprecatedClass_constructors_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [deprecatedClassFieldReader, deprecatedClassFieldName]
    wp_apply wp_readDeprecatedEntryValues_rejected validator c (structFieldRef DeprecatedCairoClass go!"Constructors" dst) slice
      v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %slice %old.constructors [Hfield Hprevious]
    · iframe Hfield Hprevious
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold DeprecatedClassFieldError
      iexists err
      iframe Herr
      ipureintro; rfl
    ipureintro; exact progress
  | @program wire why failure =>
    icases deprecatedClass_program_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    simp only [deprecatedClassFieldReader, deprecatedClassFieldName]
    ihave #Hkind := stringErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readString_rejected validator c (structFieldRef DeprecatedCairoClass go!"Program" dst) old.program
      v xs wire dq why (stringFailureKind why kinds) failure hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hkind Hfield Hc Hs
    ihave Howned := Hclose $$ %old.program Hfield
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold DeprecatedClassFieldError
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassFieldError_nonNil
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deprecatedClass_field_rejected
