import ProofCheck.ProjectionInitialize
import ProofCheck.WantedComplete

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

/-- The actual scanner specialized to any initialized projection table. The
    opening wrapper supplies the global-pointer correspondence; initialization
    supplies immutable names and the exact mask, without caller assumptions
    about allocation or cached metadata contents. -/
theorem wp_nextWanted_projection (validator : Utf8ValidatorContract (GF := GF))
    (schema : ProjectionSchema) (c st : Loc) (v : cursor.Cursor) (s : cursor.Struct)
    (seen : List Nat) (xs wire : List w8) (dq : DFrac) (d n : Nat)
    (hleft : s.left' = W64 n) (countBound : n < 2^63)
    (hs : WantedSeen (projectionNames schema).length s.seen' seen)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       globalAddr (projectionGlobal schema) ↦□ s.fields' ∗ c ↦ v ∗ st ↦ s ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ (code : w64) (result : GoInterface), RET (PairV #code #result);
       v.buf' ↦*{dq} xs ∗ ∃ (outcome : WantedOutcome) (kinds : FeltErrorKinds),
         FeltErrorGlobals kinds ∗ ⌜WantedSpec d n (projectionNames schema) seen wire outcome⌝ ∗
         WantedScanOwned c st v s xs wire kinds code result outcome }} := by
  wp_start_folded as ⟨#Hglobal, Hc, Hst, Hs⟩
  ihave #Htable := projection_initialized_table (GF := GF) schema $$ []
  · solve_pkg_init
  isimp only [ProjectionTable] at Htable
  icases Htable with ⟨%fields, #Hpublished, #Htable⟩
  ihave %agree := typedPointsto_agree_wand (globalAddr (projectionGlobal schema))
    DFrac.discard DFrac.discard fields s.fields' $$ Hpublished Hglobal
  subst fields
  isimp only [StaticFieldsShared] at Htable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %Hmask⟩
  wp_apply wp_nextWanted_complete validator c st s.fields' v s f (projectionNames schema) seen xs wire
    DFrac.discard DFrac.discard dq d n rfl hleft countBound
    (projectionNames_small schema) (projectionNames_distinct schema) Hmask hs hd depth hp hbytes
    $$ [Hc Hst Hs] as %code %result ⟨_, _, Hs, Hout⟩
  · iframe Hc Hst Hfields Hnames Hs
  iapply HΦ
  iframe Hs Hout

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_projection
