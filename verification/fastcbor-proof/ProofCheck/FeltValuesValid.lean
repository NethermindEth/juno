import ProofCheck.FeltValuesOwnership

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Every independent successful slice outcome is realized by the actual generic
    decoder, preserving null versus allocated empty and exact suffix ownership. -/
theorem wp_limbSlice_valid (c : Loc) (v : cursor.Cursor) (xs wire rest : List w8)
    (values : Option (List (GoArray w64 4))) (dq : DFrac) (d : Nat)
    (valid : FeltValuesSpec d wire (.success values rest)) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ (out : GoSlice), RET (PairV #out #interface.nil);
       v.buf' ↦*{dq} xs ∗ FeltValuesSuccess c v xs wire out values rest }} := by
  wp_start_folded as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  cases valid with
  | nullish head tail hn =>
    have hi := input_suffix_head xs rest _ _ head hbytes
    have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
    wp_apply wp_limbSlice_null c v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hs
    isimp only [FeltValuesSuccess, FeltValuesResult]
    isplitr
    · ipureintro; trivial
    iexists [head]
    isimp only [List.length_cons, List.length_nil]
    iframe Hc
    ipureintro
    exact ⟨rfl, by simp, Hnext⟩
  | @array head payload item rest argument values count enc major size fit depth children valid =>
    have harg : argument = W64 count := by word
    subst argument
    have Hhead := feltValues_head_input v xs payload (item ++ rest) head Hlen.1 hp
      (by simpa only [List.append_assoc] using hbytes)
    have Hnext := input_consumption xs ((head :: payload) ++ item) rest v.pos' v.buf'.len hp Hlen.1 hbytes
    by_cases empty : count = 0
    · subst count
      cases valid
      simp only [List.append_nil, List.nil_append] at hbytes Hhead Hnext ⊢
      wp_apply wp_limbSlice_empty c v xs payload rest head dq enc major (by omega) hp.1
        (by omega) Hhead.2.1 Hhead.2.2 $$ [Hc Hs] as %out ⟨Hc, Hs, Hout, Hcap, %Hnn⟩
      · iframe Hc Hs
      iapply HΦ
      iframe Hs
      isimp only [FeltValuesSuccess, FeltValuesResult]
      isplitl [Hout Hcap]
      · iframe Hout Hcap; ipureintro; exact Hnn
      iexists (head :: payload)
      have hpos : v.pos' + W64 (1 + payload.length) = v.pos' + W64 (head :: payload).length := by simp only [List.length_cons]; word
      isimp only [hpos] at Hc
      iframe Hc
      ipureintro
      exact ⟨rfl, by simp, Hnext⟩
    · wp_apply wp_limbSlice_nonempty_valid c v xs payload item rest values head dq d count enc major
        (by omega) fit valid hd (by omega) hp hbytes $$ [Hc Hs] as %out ⟨Hout, Hcap, Hc, Hs, %Hnext'⟩
      · iframe Hc Hs
      iapply HΦ
      iframe Hs
      isimp only [FeltValuesSuccess, FeltValuesResult]
      isplitl [Hout Hcap]
      · iframe Hout Hcap; ipureintro; exact Hnext'.1
      iexists ((head :: payload) ++ item)
      have hpos : v.pos' + W64 (1 + payload.length + item.length) = v.pos' + W64 ((head :: payload) ++ item).length := by
        simp only [List.length_append, List.length_cons]; word
      isimp only [hpos] at Hc
      iframe Hc
      ipureintro
      exact ⟨rfl, by simp, Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_valid
