import ProofCheck.FeltValuesRejected
import ProofCheck.InitializationErrors

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

def FeltValuesResult (out : GoSlice) : Option (List (GoArray w64 4)) → IProp GF
  | none => iprop(⌜out = slice.nil⌝)
  | some values => iprop(out ↦* values ∗ ownSliceCap (GoArray w64 4) out (DFrac.own 1) ∗ ⌜out.ptr ≠ null⌝)

/-- A present empty slice remains distinguishable from null. -/
theorem feltValuesResult_null_iff (out : GoSlice) (values : Option (List (GoArray w64 4))) :
    FeltValuesResult (GF := GF) out values ⊢ ⌜out.ptr = null ↔ values = none⌝ := by
  cases values with
  | none => isimp only [FeltValuesResult]; iintro %h; ipureintro; simp [h, slice.nil]
  | some values => isimp only [FeltValuesResult]; iintro ⟨_, _, %h⟩; ipureintro; simp [h]

def FeltValuesFailureError (err : Loc) (pos : w64) (reason : FeltValuesFailureReason)
    (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .expected reason => ExpectedFailureError err pos (W8 128) reason (expectedFailureKind reason kinds)
  | .count count left => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kinds.unsupportedKind pos
        (go!"count %d does not fit the %d bytes left at five bytes per felt") args ∗
      args ↦* [interface.mkOk go.uint64 #(W64 count), interface.mkOk go.int #(W64 left)])
  | .depth => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kinds.depthKind pos (go!"nested more than %d levels") args ∗
      args ↦* [interface.mkOk go.int #(W64 32)])
  | .element reason => FeltFailureError err pos reason (feltFailureKind reason kinds)

/-- The successful slice owns its complete backing allocation and identifies
    exactly the consumed prefix; the original input ownership is framed outside. -/
def FeltValuesSuccess (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (out : GoSlice)
    (values : Option (List (GoArray w64 4))) (rest : List w8) : IProp GF := iprop(
  FeltValuesResult out values ∗ ∃ item : List w8,
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜wire = item ++ rest ∧ 0 < item.length ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝)

/-- Positional facts needed by each array-head branch, derived from the exact
    unread bytes rather than assumed separately by the caller. -/
theorem feltValues_head_input (v : cursor.Cursor) (xs payload tail : List w8) (head : w8)
    (hlen : xs.length = sint.nat v.buf'.len)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ tail) :
    sint.Z v.pos' + 1 + payload.length + tail.length = sint.Z v.buf'.len ∧
    xs[(sint.Z v.pos').toNat]? = some head ∧
    subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ tail := by
  have hlength := congrArg List.length hbytes
  simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
    List.length_append, List.length_cons] at hlength
  have htotal : sint.Z v.pos' + 1 + payload.length + tail.length = sint.Z v.buf'.len := by
    simp only [sint.nat, sint.Z] at hlen hlength hp ⊢
    omega
  refine ⟨htotal, input_suffix_head xs (payload ++ tail) _ _ head hbytes, ?_⟩
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  rw [hp1]
  exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append] using hbytes)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValuesResult_null_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValues_head_input
