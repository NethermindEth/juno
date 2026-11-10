import ProofCheck.HeaderFeltComplete
import ProofCheck.HeaderScalars
import ProofCheck.HeaderSignatures
import ProofCheck.HeaderGasReady

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

def headerFeltKey : HeaderFeltField → HeaderField
  | .hash => .hash
  | .parentHash => .parentHash
  | .globalStateRoot => .globalStateRoot
  | .sequencerAddress => .sequencerAddress
  | .gasETH => .gasETH
  | .gasSTRK => .gasSTRK

def headerScalarKey : HeaderScalarField → HeaderField
  | .number => .number
  | .transactions => .transactionCount
  | .events => .eventCount
  | .timestamp => .timestamp
  | .mode => .mode

/-- Successful field calls expose the value specified by their input relation,
    rather than an existentially classified value. They also carry the updated
    seen mask required by the next Header map iteration. No field-reader WP is
    assumed: each call is composed from its checked production contract. -/
theorem wp_headerFelt_success (c dst base : Loc) (h : Header) (old : HeaderValue)
    (field : HeaderFeltField) (seen : w64) (v : cursor.Cursor) (xs wire rem : List w8)
    (value : Option (GoArray w64 4)) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32) (valid : FeltValid d wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ h ∗ HeaderBody h old ∗ HeaderGasReady base h old seen ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef Header (headerFeltName field) dst)))
    {{ (p : Loc), RET #interface.nil;
       dst ↦ headerSetFeltPointer field h p ∗
       HeaderBody (headerSetFeltPointer field h p) (headerSetFeltValue field old value) ∗
       HeaderGasReady base (headerSetFeltPointer field h p) (headerSetFeltValue field old value)
         (headerKeySeen seen (some (headerFeltKey field))) ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hh, Hbody, Hready, Hc, Hs⟩
  wp_apply wp_headerFelt_valid c dst h old field v xs wire rem value dq d hd hdepth valid hp hbytes
    $$ [Hh Hbody Hc Hs] as %p ⟨Hh, Hbody, Hc, Hs, %Hnext⟩
  · iframe Hh Hbody Hc Hs
  ihave Hready := headerGasReady_mark base h old seen (some (headerFeltKey field)) $$ Hready
  iapply HΦ
  isimp only [headerGasReady_set_felt]
  iframe Hh Hbody Hready Hc Hs
  ipureintro
  exact Hnext

/-- All five scalar branches, including null preserving the prior scalar, now
    have a success contract in the same private ownership state as GasPrice. -/
theorem wp_headerScalar_success (c dst base : Loc) (h : Header) (old : HeaderValue)
    (field : HeaderScalarField) (seen : w64) (v : cursor.Cursor) (xs wire rem : List w8)
    (value : w64) (dq : DFrac) (valid : ScalarValid (headerScalarValue field old) wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ h ∗ HeaderBody h old ∗ HeaderGasReady base h old seen ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (if headerScalarMode field then (@! cborReadMode) else (@! cborReadUint64))) (Val #c))
        (Val #(structFieldRef Header (headerScalarName field) dst)))
    {{ RET #interface.nil;
       dst ↦ headerSetScalarWord field h value ∗
       HeaderBody (headerSetScalarWord field h value) (headerSetScalarValue field old value) ∗
       HeaderGasReady base (headerSetScalarWord field h value) (headerSetScalarValue field old value)
         (headerKeySeen seen (some (headerScalarKey field))) ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hh, Hbody, Hready, Hc, Hs⟩ HΦ
  isimp only [HeaderBody] at Hbody
  icases Hbody with ⟨%Hscalars, Hchildren⟩
  ihave Hbody : HeaderBody h old $$ [Hchildren]
  · unfold HeaderBody
    iframe Hchildren
    ipureintro
    exact Hscalars
  have hword := headerScalars_word field h old Hscalars
  have hvalid : ScalarValid (headerScalarWord field h) wire value := by simpa only [hword] using valid
  icases headerScalar_acc dst h field $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readScalar_valid (headerScalarMode field) c (structFieldRef Header (headerScalarName field) dst)
    (headerScalarWord field h) v xs wire rem value dq hvalid hp hbytes $$ [Hfield Hc Hs]
    with ⟨Hfield, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Hh := Hclose $$ %value Hfield
  ihave Hbody := headerBody_set_scalar h old field value $$ Hbody
  ihave Hready := headerGasReady_mark base h old seen (some (headerScalarKey field)) $$ Hready
  iapply HΦ
  isimp only [headerGasReady_set_scalar]
  iframe Hh Hbody Hready Hc Hs
  ipureintro
  exact Hnext

/-- The nested signature branch publishes its specified nullable/list value,
    retains every other Header field, and marks only the signature key. -/
theorem wp_headerSignatures_success (c dst base : Loc) (h : Header) (old : HeaderValue)
    (seen : w64) (v : cursor.Cursor) (xs wire rem : List w8) (value : Option (List SignatureValue))
    (dq : DFrac) (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (valid : SignaturesValid d wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ h ∗ HeaderBody h old ∗ HeaderGasReady base h old seen ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSignatures)) (Val #c)) (Val #(structFieldRef Header go!"Signatures" dst)))
    {{ (p : GoSlice), RET #interface.nil;
       dst ↦ {h with Signatures' := p} ∗ HeaderBody {h with Signatures' := p} {old with signatures := value} ∗
       HeaderGasReady base {h with Signatures' := p} {old with signatures := value}
         (headerKeySeen seen (some .signatures)) ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hh, Hbody, Hready, Hc, Hs⟩
  icases headerSignatures_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readSignatures_valid c (structFieldRef Header go!"Signatures" dst) h.Signatures'
    v xs wire rem value dq d hd hdepth valid hp hbytes $$ [Hfield Hc Hs]
    as %p ⟨Hfield, Hvalue, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Hh := Hclose $$ %p Hfield
  ihave Hbody := headerBody_set_signatures h old p value $$ [Hbody Hvalue]
  · iframe Hbody Hvalue
  ihave Hready := headerGasReady_mark base h old seen (some .signatures) $$ Hready
  iapply HΦ
  isimp only [headerGasReady_set_signatures]
  iframe Hh Hbody Hready Hc Hs
  ipureintro
  exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerFelt_success
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerScalar_success
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerSignatures_success
