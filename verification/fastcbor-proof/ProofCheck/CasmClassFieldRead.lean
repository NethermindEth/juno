import ProofCheck.CasmClassOwnership

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

def casmClassFieldCall (field : CasmClassField) (c dst : Loc) : Expr :=
  match field with
  | .bytecode => (App (App (Val (@! cborReadFeltValues)) (Val #c))
      (Val #(structFieldRef CasmClass go!"Bytecode" dst)))
  | .pythonicHints => (App (App (Val (@! cborReadRawMessage)) (Val #c))
      (Val #(structFieldRef CasmClass go!"PythonicHints" dst)))
  | .compilerVersion => (App (App (Val (@! cborReadString)) (Val #c))
      (Val #(structFieldRef CasmClass go!"CompilerVersion" dst)))
  | .hints => (App (App (Val (@! cborReadRawMessage)) (Val #c))
      (Val #(structFieldRef CasmClass go!"Hints" dst)))
  | .prime => (App (App (Val (@! cborReadBigIntPointer)) (Val #c))
      (Val #(structFieldRef CasmClass go!"Prime" dst)))
  | .external => (App (App (Val (@! cborReadCasmEntryPointValues)) (Val #c))
      (Val #(structFieldRef CasmClass go!"External" dst)))
  | .l1Handler => (App (App (Val (@! cborReadCasmEntryPointValues)) (Val #c))
      (Val #(structFieldRef CasmClass go!"L1Handler" dst)))
  | .constructor => (App (App (Val (@! cborReadCasmEntryPointValues)) (Val #c))
      (Val #(structFieldRef CasmClass go!"Constructor" dst)))
  | .segments => (App (App (Val (@! cborReadSegmentLengths)) (Val #c))
      (Val #(structFieldRef CasmClass go!"BytecodeSegmentLengths" dst)))

/-- All fields retain the actual allocated diagnostics. BigInt errors are
    indexed by their reported position, including tag-payload offsets. -/
def CasmClassFieldFailureOwned (err : Loc) (pos : w64) : CasmClassFieldFailure → IProp GF
  | .felts reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      FeltValuesFailureError err pos reason kinds)
  | .bytes reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ BytesError err pos kinds reason)
  | .string reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      StringFailureError err pos reason (stringFailureKind reason kinds))
  | .bigint reason => iprop(∃ kinds : BigIntErrorKinds, BigIntErrorGlobals kinds ∗
      BigIntFailureError err (pos - W64 (bigIntDiagnosticOffset reason)) reason kinds)
  | .entries reason => CasmEntryValuesFailureOwned err pos reason
  | .segment reason => SegmentDiagnosticOwned err pos reason

theorem wp_casmClass_field_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : CasmClassValue) (field : CasmClassField)
    (v : cursor.Cursor) (xs item rest : List w8) (dq : DFrac) (d : Nat)
    (valid : CasmClassFieldValid d field old item rest final) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       CasmClassOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (casmClassFieldCall (GF := GF) field c dst)
    {{ RET #interface.nil; CasmClassOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  cases valid with
  | @bytecode old item rest value valid =>
    simp only [casmClassFieldCall]
    icases casmClass_bytecode_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readFeltValues_valid c _ previous v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @pythonicHints old item rest value valid =>
    simp only [casmClassFieldCall]
    icases casmClass_pythonicHints_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readRawMessage_valid c _ previous v xs item rest value dq valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @compilerVersion old item rest value valid =>
    simp only [casmClassFieldCall]
    icases casmClass_compilerVersion_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    wp_apply wp_readString_valid validator c _ old.compilerVersion v xs item rest value dq valid hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %value Hfield
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @hints old item rest value valid =>
    simp only [casmClassFieldCall]
    icases casmClass_hints_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readRawMessage_valid c _ previous v xs item rest value dq valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @prime old item rest value valid =>
    simp only [casmClassFieldCall]
    icases casmClass_prime_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readBigIntPointer_valid c _ previous v xs item rest value dq valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @external old item rest value valid =>
    simp only [casmClassFieldCall]
    icases casmClass_external_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readCasmEntryValues_valid validator c _ previous v xs (item ++ rest) rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    isimp only [List.length_append, Nat.add_sub_cancel] at Hc
    simp only [List.length_append, Nat.add_sub_cancel] at Hnext
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @l1Handler old item rest value valid =>
    simp only [casmClassFieldCall]
    icases casmClass_l1Handler_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readCasmEntryValues_valid validator c _ previous v xs (item ++ rest) rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    isimp only [List.length_append, Nat.add_sub_cancel] at Hc
    simp only [List.length_append, Nat.add_sub_cancel] at Hnext
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @constructor old item rest value valid =>
    simp only [casmClassFieldCall]
    icases casmClass_constructor_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readCasmEntryValues_valid validator c _ previous v xs (item ++ rest) rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    isimp only [List.length_append, Nat.add_sub_cancel] at Hc
    simp only [List.length_append, Nat.add_sub_cancel] at Hnext
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @segments old item rest value valid =>
    simp only [casmClassFieldCall]
    icases casmClass_segments_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    wp_apply wp_readSegment_record_valid validator c _ old.segments value v xs item rest dq d depth valid hd hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %value Hfield
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext

theorem wp_casmClass_field_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old retained : CasmClassValue) (field : CasmClassField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat)
    (reason : CasmClassFieldFailure)
    (failure : CasmClassFieldRejected d field old wire retained consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       CasmClassOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (casmClassFieldCall (GF := GF) field c dst)
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       CasmClassOwned dst retained ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ CasmClassFieldFailureOwned err (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bounds := casmClassField_rejected_bounds _ _ _ _ _ _ _ _ _ failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have progress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by
    constructor; · word
    constructor; · word
    constructor <;> word
  cases failure with
  | @bytecode old wire offset why failure =>
    simp only [casmClassFieldCall]
    icases casmClass_bytecode_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readFeltValues_rejected c _ previous v xs wire dq d offset why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %previous %old.bytecode [Hfield Hprevious]
    · iframe Hfield Hprevious
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmClassFieldFailureOwned
      iexact Herr
    ipureintro; simpa using progress
  | @pythonicHints old wire why failure =>
    simp only [casmClassFieldCall]
    icases casmClass_pythonicHints_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readRawMessage_rejected c _ previous v xs wire dq why kinds failure hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hglobals Hfield Hc Hs
    ihave Howned := Hclose $$ %previous %old.pythonicHints [Hfield Hprevious]
    · iframe Hfield Hprevious
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmClassFieldFailureOwned
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress
  | @compilerVersion old wire why failure =>
    simp only [casmClassFieldCall]
    icases casmClass_compilerVersion_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    ihave #Hkind := stringErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readString_rejected validator c _ old.compilerVersion v xs wire dq why (stringFailureKind why kinds) failure hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hkind Hfield Hc Hs
    ihave Howned := Hclose $$ %old.compilerVersion Hfield
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmClassFieldFailureOwned
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress
  | @hints old wire why failure =>
    simp only [casmClassFieldCall]
    icases casmClass_hints_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readRawMessage_rejected c _ previous v xs wire dq why kinds failure hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hglobals Hfield Hc Hs
    ihave Howned := Hclose $$ %previous %old.hints [Hfield Hprevious]
    · iframe Hfield Hprevious
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmClassFieldFailureOwned
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress
  | @prime old wire why failure =>
    simp only [casmClassFieldCall]
    icases casmClass_prime_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    ihave HbigGlobals := cursor_initialized_bigInt_globals (GF := GF) $$ []
    · solve_pkg_init
    icases HbigGlobals with ⟨%bigKinds, #HbigGlobals⟩
    wp_apply wp_readBigIntPointer_rejected c _ previous v xs wire dq why bigKinds failure hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe HbigGlobals Hfield Hc Hs
    ihave Howned := Hclose $$ %previous %old.prime [Hfield Hprevious]
    · iframe Hfield Hprevious
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmClassFieldFailureOwned
      iexists bigKinds
      have back : v.pos' + W64 (bigIntDiagnosticOffset why) - W64 (bigIntDiagnosticOffset why) = v.pos' := by word
      isimp only [back]
      iframe HbigGlobals Herr
    ipureintro; simpa using progress
  | @external old wire consumed offset extraDepth why failure =>
    simp only [casmClassFieldCall]
    icases casmClass_external_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readCasmEntryValues_rejected validator c _ previous v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %previous %old.external [Hfield Hprevious]
    · iframe Hfield Hprevious
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmClassFieldFailureOwned
      iexact Herr
    ipureintro; simpa using progress
  | @l1Handler old wire consumed offset extraDepth why failure =>
    simp only [casmClassFieldCall]
    icases casmClass_l1Handler_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readCasmEntryValues_rejected validator c _ previous v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %previous %old.l1Handler [Hfield Hprevious]
    · iframe Hfield Hprevious
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmClassFieldFailureOwned
      iexact Herr
    ipureintro; simpa using progress
  | @constructor old wire consumed offset extraDepth why failure =>
    simp only [casmClassFieldCall]
    icases casmClass_constructor_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readCasmEntryValues_rejected validator c _ previous v xs wire dq d consumed offset extraDepth why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %previous %old.constructor [Hfield Hprevious]
    · iframe Hfield Hprevious
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmClassFieldFailureOwned
      iexact Herr
    ipureintro; simpa using progress
  | @segments old wire value consumed offset extraDepth why failure =>
    simp only [casmClassFieldCall]
    icases casmClass_segments_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    wp_apply wp_readSegment_record_rejected_allocated validator c _ old.segments value v xs wire dq d consumed offset depth extraDepth why failure hd hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %value Hfield
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold CasmClassFieldFailureOwned
      iexact Herr
    ipureintro; simpa using progress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_casmClass_field_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_casmClass_field_rejected
