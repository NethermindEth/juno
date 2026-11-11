import ProofCheck.SierraEntriesAllocated
import ProofCheck.SierraClassOwnership

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

def sierraClassFieldCall (field : SierraClassField) (c dst : Loc) : Expr :=
  match field with
  | .abi => (App (App (Val (@! cborReadString)) (Val #c))
      (Val #(structFieldRef SierraClass go!"Abi" dst)))
  | .abiHash => (App (App (Val (@! cborReadFelt)) (Val #c))
      (Val #(structFieldRef SierraClass go!"AbiHash" dst)))
  | .entries => (App (App (Val (@! cborReadSierraEntryPointsByType)) (Val #c))
      (Val #(structFieldRef SierraClass go!"EntryPoints" dst)))
  | .program => (App (App (Val (@! cborReadFeltValues)) (Val #c))
      (Val #(structFieldRef SierraClass go!"Program" dst)))
  | .programHash => (App (App (Val (@! cborReadFelt)) (Val #c))
      (Val #(structFieldRef SierraClass go!"ProgramHash" dst)))
  | .version => (App (App (Val (@! cborReadString)) (Val #c))
      (Val #(structFieldRef SierraClass go!"SemanticVersion" dst)))
  | .compiled => (App (App (Val (@! cborReadCasmClassPointer)) (Val #c))
      (Val #(structFieldRef SierraClass go!"Compiled" dst)))

/-- Preserve initialized diagnostics and privately owned failed Compiled values.
    Entry-point errors retain the child record's exact allocated interface. -/
def SierraClassFieldFailureOwned (err : Loc) (pos : w64) : SierraClassFieldFailure → IProp GF
  | .string reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      StringFailureError err pos reason (stringFailureKind reason kinds))
  | .felt reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      FeltFailureError err pos reason (feltFailureKind reason kinds))
  | .entries reason => SierraEntriesFailureOwned (interface.mkOk (go.PointerType cursor.DecodeError.ty) #err) pos reason
  | .felts reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      FeltValuesFailureError err pos reason kinds)
  | .compiled retained reason => iprop((∃ temporary : Loc, CasmClassOwned temporary retained) ∗
      CasmClassFailureOwned err pos reason)

theorem wp_sierraClass_field_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : SierraClassValue) (field : SierraClassField)
    (v : cursor.Cursor) (xs item rest : List w8) (dq : DFrac) (d : Nat)
    (valid : SierraClassFieldValid d field old item rest final) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SierraClassOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (sierraClassFieldCall (GF := GF) field c dst)
    {{ RET #interface.nil; SierraClassOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  cases valid with
  | @abi old item rest value valid =>
    simp only [sierraClassFieldCall]
    icases sierraClass_abi_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    wp_apply wp_readString_valid validator c _ old.abi v xs item rest value dq valid hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %value Hfield
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @abiHash old item rest value valid =>
    simp only [sierraClassFieldCall]
    icases sierraClass_abiHash_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readFelt_valid c _ previous v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @entries old item rest value valid =>
    simp only [sierraClassFieldCall]
    icases sierraClass_entries_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    wp_apply wp_readSierraEntries_record_valid validator c _ old.entries value v xs item rest dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %value Hfield
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @program old item rest value valid =>
    simp only [sierraClassFieldCall]
    icases sierraClass_program_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readFeltValues_valid c _ previous v xs item rest value dq d valid hd depth hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @programHash old item rest value valid =>
    simp only [sierraClassFieldCall]
    icases sierraClass_programHash_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readFelt_valid c _ previous v xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @version old item rest value valid =>
    simp only [sierraClassFieldCall]
    icases sierraClass_version_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    wp_apply wp_readString_valid validator c _ old.version v xs item rest value dq valid hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %value Hfield
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @compiled old item rest value valid =>
    simp only [sierraClassFieldCall]
    icases sierraClass_compiled_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readCasmClassPointer_valid validator c _ previous v xs item rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext

theorem wp_sierraClass_field_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old retained : SierraClassValue) (field : SierraClassField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat)
    (reason : SierraClassFieldFailure)
    (failure : SierraClassFieldRejected d field old wire retained consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SierraClassOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (sierraClassFieldCall (GF := GF) field c dst)
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       SierraClassOwned dst retained ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ SierraClassFieldFailureOwned err (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bounds := sierraClassField_rejected_bounds _ _ _ _ _ _ _ _ _ failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have progress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by
    constructor; · word
    constructor; · word
    constructor <;> word
  cases failure with
  | @abi old wire why failure =>
    simp only [sierraClassFieldCall]
    icases sierraClass_abi_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    ihave #Hkind := stringErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readString_rejected validator c _ old.abi v xs wire dq why (stringFailureKind why kinds) failure hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hkind Hfield Hc Hs
    ihave Howned := Hclose $$ %old.abi Hfield
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold SierraClassFieldFailureOwned
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress
  | @abiHash old wire offset why failure =>
    simp only [sierraClassFieldCall]
    icases sierraClass_abiHash_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c _ previous v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hkind Hfield Hc Hs
    ihave Howned := Hclose $$ %previous %old.abiHash [Hfield Hprevious]
    · iframe Hfield Hprevious
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold SierraClassFieldFailureOwned
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress
  | @entries old wire value consumed offset extraDepth why failure =>
    simp only [sierraClassFieldCall]
    icases sierraClass_entries_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    wp_apply wp_readSierraEntries_record_rejected validator c _ old.entries value v xs wire dq d consumed offset extraDepth why failure hd hp hbytes
      $$ [Hfield Hc Hs] as %result ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave %Hallocated := sierraEntriesFailureOwned_allocated result _ why $$ Herr
    obtain ⟨err, rfl⟩ := Hallocated
    ihave Howned := Hclose $$ %value Hfield
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold SierraClassFieldFailureOwned
      iexact Herr
    ipureintro; simpa using progress
  | @program old wire offset why failure =>
    simp only [sierraClassFieldCall]
    icases sierraClass_program_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readFeltValues_rejected c _ previous v xs wire dq d offset why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %previous %old.program [Hfield Hprevious]
    · iframe Hfield Hprevious
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold SierraClassFieldFailureOwned
      iexact Herr
    ipureintro; simpa using progress
  | @programHash old wire offset why failure =>
    simp only [sierraClassFieldCall]
    icases sierraClass_programHash_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c _ previous v xs wire dq d offset why (feltFailureKind why kinds) failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hkind Hfield Hc Hs
    ihave Howned := Hclose $$ %previous %old.programHash [Hfield Hprevious]
    · iframe Hfield Hprevious
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold SierraClassFieldFailureOwned
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress
  | @version old wire why failure =>
    simp only [sierraClassFieldCall]
    icases sierraClass_version_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    ihave #Hkind := stringErrorGlobals_lookup kinds why $$ Hglobals
    wp_apply wp_readString_rejected validator c _ old.version v xs wire dq why (stringFailureKind why kinds) failure hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hkind Hfield Hc Hs
    ihave Howned := Hclose $$ %old.version Hfield
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold SierraClassFieldFailureOwned
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress
  | @compiled old wire retained consumed offset extraDepth why failure =>
    simp only [sierraClassFieldCall]
    icases sierraClass_compiled_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
    wp_apply wp_readCasmClassPointer_rejected validator c _ previous v xs wire retained dq d consumed offset extraDepth why failure hd hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hprivate, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %previous %old.compiled [Hfield Hprevious]
    · iframe Hfield Hprevious
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr Hprivate]
    · unfold SierraClassFieldFailureOwned
      isplitl [Hprivate]
      · iexact Hprivate
      · iexact Herr
    ipureintro; simpa using progress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_sierraClass_field_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_sierraClass_field_rejected
