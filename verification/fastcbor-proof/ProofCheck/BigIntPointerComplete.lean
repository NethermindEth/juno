import ProofCheck.BigIntPointerRead

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

def BigIntPointerReadOutcome (c : Loc) (dst : Loc) (old : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : BigIntErrorKinds) (result : GoInterface) : BigIntPointerResult → IProp GF
  | .success body rest => iprop(∃ (consumed : Nat) (out : Loc),
      dst ↦ out ∗ BigIntPointerOwned out body ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure reason => iprop(dst ↦ old ∗ c ↦ v ∗ ∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      BigIntFailureError err v.pos' reason kinds)

/-- The actual typed BigIntPointer decoder for every byte input and owned initial
    pointer cell. The result satisfies the independent specification uniquely; no
    additional UTF-8, integer magnitude, allocation or initialization premise is introduced. -/
theorem wp_readBigIntPointer_complete (c : Loc) (dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadBigIntPointer)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ (kinds : BigIntErrorKinds) (outcome : BigIntPointerResult),
       BigIntErrorGlobals kinds ∗ ⌜BigIntPointerSpec wire outcome ∧ ∀ other, BigIntPointerSpec wire other → other = outcome⌝ ∗
       BigIntPointerReadOutcome c dst old v xs wire kinds result outcome }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_bigInt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  obtain ⟨outcome, spec, unique⟩ := bigIntPointerSpec_total_unique wire
  cases outcome with
  | success body rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_readBigIntPointer_valid c dst old v xs item rest body dq valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] as %out ⟨Hdst, Hout, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, BigIntPointerResult.success body rest
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [BigIntPointerReadOutcome]
    iexists item.length, out
    iframe Hdst Hout Hc
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure reason =>
    wp_apply wp_readBigIntPointer_rejected c dst old v xs wire dq reason kinds spec hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hglobals Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, BigIntPointerResult.failure reason
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [BigIntPointerReadOutcome]
    iframe Hdst Hc
    iexists err
    iframe Herr
    ipureintro; rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readBigIntPointer_complete
