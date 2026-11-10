import ProofCheck.GasPriceMapComplete

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

/-- The actual record reader realizes every successful independent GasPrice
    encoding, with nullish preservation and exact owned output/suffix. This
    remains conditional on the unresolved production UTF-8 validator proof. -/
theorem wp_readGasPrice_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst fields : Loc) (g : GasPrice) (initial final : GasPriceValue)
    (v : cursor.Cursor) (f : cursor.Fields) (xs wire rem : List w8)
    (kind : GoInterface) (qf qo qn dq : DFrac) (d : Nat)
    (valid : GasRecordValid d initial wire final) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ g ∗ GasPriceChildren g initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #dst))
    {{ (out : GasPrice), RET #interface.nil;
       dst ↦ out ∗ GasPriceChildren out final ∗ c ↦ {v with pos' := v.pos' + W64 wire.length} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, #Hglobal, #Hkind, Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 bytes
  cases valid with
  | @nullish head nullish =>
    have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head bytes
    wp_apply wp_readGasPrice_nullish c dst fields g v xs head dq hp.1 hi nullish
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hglobal Hdst Hc Hs
    isimp only [List.length_singleton, Int.natCast_one] at HΦ
    iapply HΦ
    iframe Hdst Hchildren Hc Hs Hfields Horder Hnames
    ipureintro
    simpa only [List.length_singleton, Int.natCast_one] using Hnext
  | @map head payload body argument count final enc major countEq depth pairs =>
    have argumentEq : argument = W64 count := by word
    subst argument
    wp_apply wp_readGasPrice_map_valid validator c dst fields g initial v f xs payload body rem head
      kind qf qo qn dq d count final enc major hd depth pairs hp bytes
      $$ [Hdst Hchildren Hc Hs Hfields Horder Hnames] as %out ⟨Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames, %Hadvance⟩
    · iframe Hglobal Hkind Hdst Hchildren Hc Hs Hfields Horder Hnames
    have length : ((head :: payload) ++ body).length = 1 + payload.length + body.length := by simp; omega
    isimp only [length, Int.natCast_add, Int.natCast_one] at HΦ
    iapply HΦ
    iframe Hdst Hchildren Hc Hs Hfields Horder Hnames
    ipureintro
    simpa only [length, Int.natCast_add, Int.natCast_one] using Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_valid
