import ProofCheck.Utf8Contract
import ProofCheck.StringConversion
import ProofCheck.StringSpec

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

/-- Full text success through actual payload extraction, validation and copying.
    The production validator WP is an explicit, currently unresolved premise. -/
theorem wp_string_text (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs payload body rem : List w8) (head : w8) (dq : DFrac)
    (enc : HeadArgument head payload (W64 body.length)) (major : head &&& W8 224 = W8 96)
    (valid : Utf8Valid body)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"String")) (Val #()))
    {{ RET (PairV #body #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + body.length)) = sint.Z v.pos' + 1 + payload.length + body.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + body.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 bytes
  have length : ((head :: payload) ++ body).length = 1 + payload.length + body.length := by simp; omega
  rw [length] at Hnext
  simp only [Int.natCast_add, Int.natCast_one] at Hnext
  wp_auto
  rw [← major]
  wp_apply wp_payload_encoded c v xs payload body rem head dq enc hp bytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hrange⟩
  · iframe Hc Hs
  wp_apply wp_utf8_contract_range validator v.buf' xs body
    (v.pos' + W64 (1 + payload.length)) (v.pos' + W64 (1 + payload.length + body.length)) dq
    ⟨Hrange.1, Hrange.2.1, Hrange.2.2.1⟩ Hrange.2.2.2.1 $$ [Hs] as %ok ⟨Hs, %Hvalid⟩
  · iframe Hs
  have accepted : ok = true := Hvalid.mpr valid
  subst ok
  wp_auto
  wp_apply wp_bytes_to_string_range v.buf' xs body
    (v.pos' + W64 (1 + payload.length)) (v.pos' + W64 (1 + payload.length + body.length)) dq
    ⟨Hrange.1, Hrange.2.1, Hrange.2.2.1⟩ Hrange.2.2.2.1 $$ [Hs] with Hs
  · iframe Hs
  iapply HΦ
  iframe Hc Hs
  ipureintro; exact ⟨by omega, Hnext.2⟩

/-- Every malformed UTF-8 payload takes the real rollback/error path, provided
    the actual validator satisfies its explicit contract. -/
theorem wp_string_invalid_utf8 (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs payload body rem : List w8) (head : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 body.length)) (major : head &&& W8 224 = W8 96)
    (invalid : ¬ Utf8Valid body)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"String")) (Val #()))
    {{ (err : Loc), RET (PairV #(go!"") #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"text string is not valid UTF-8") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  rw [← major]
  wp_apply wp_payload_encoded c v xs payload body rem head dq enc hp bytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hrange⟩
  · iframe Hc Hs
  wp_apply wp_utf8_contract_range validator v.buf' xs body
    (v.pos' + W64 (1 + payload.length)) (v.pos' + W64 (1 + payload.length + body.length)) dq
    ⟨Hrange.1, Hrange.2.1, Hrange.2.2.1⟩ Hrange.2.2.2.1 $$ [Hs] as %ok ⟨Hs, %Hvalid⟩
  · iframe Hs
  have rejected : ok = false := by cases ok <;> simp_all
  subst ok
  wp_auto
  wp_apply wp_fail c v kind (go!"text string is not valid UTF-8") slice.nil
    $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_string_text
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_string_invalid_utf8
