import ProofCheck.BigIntUnique
import ProofCheck.BigIntComplete

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

def BigIntReadOutcome (c : Loc) (value : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : BigIntErrorKinds) (result : GoInterface) : BigIntResult → IProp GF
  | .success body rest => iprop(∃ consumed : Nat,
      BigMagnitudeOwned value body ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure reason => iprop(c ↦ v ∗ ∃ err : Loc,
      ⌜value = null ∧ result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      BigIntFailureError err v.pos' reason kinds)

/-- The actual public BigInt method returns the independently unique result
    on every input, including normalized owned values and exact rollback/errors. -/
theorem wp_bigInt_complete_unique (c : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (value : Loc) (result : GoInterface), RET (PairV #value #result);
       v.buf' ↦*{dq} xs ∗ ∃ (kinds : BigIntErrorKinds) (outcome : BigIntResult),
       BigIntErrorGlobals kinds ∗ ⌜BigIntSpec wire outcome ∧ ∀ other, BigIntSpec wire other → other = outcome⌝ ∗
       BigIntReadOutcome c value v xs wire kinds result outcome }} := by
  wp_start_folded as ⟨Hc, Hs⟩
  ihave Hglobals := cursor_initialized_bigInt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  obtain ⟨outcome, spec, unique⟩ := bigIntSpec_total_unique wire
  cases outcome with
  | success body rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_bigInt_valid c v xs item rest body dq valid hp (hbytes.trans split)
      $$ [Hc Hs] as %out ⟨Hc, Hs, Hout, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, BigIntResult.success body rest
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [BigIntReadOutcome]
    iexists item.length
    iframe Hout Hc
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure reason =>
    wp_apply wp_bigInt_rejected c v xs wire dq reason kinds spec hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hglobals Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, BigIntResult.failure reason
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [BigIntReadOutcome]
    iframe Hc
    iexists err
    iframe Herr
    ipureintro; exact ⟨trivial, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_complete_unique
