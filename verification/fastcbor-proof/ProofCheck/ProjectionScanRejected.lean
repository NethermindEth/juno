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

/-- A rejected scan on an initialized projection table, including arbitrary ignored
    prefixes, exact retained state and a separately positioned owned diagnostic.
    The actual NextWanted proof retains its explicit UTF-8 validator dependency. -/
theorem wp_nextWanted_projection_rejected (validator : Utf8ValidatorContract (GF := GF))
    (schema : ProjectionSchema) (c st : Loc) (v : cursor.Cursor) (s : cursor.Struct)
    (seen : List Nat) (xs wire : List w8) (dq : DFrac) (d n consumed offset left : Nat)
    (reason : WantedFailureReason) (kinds : FeltErrorKinds)
    (hleft : s.left' = W64 n) (countBound : n < 2^63)
    (hs : WantedSeen (projectionNames schema).length s.seen' seen)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (failure : WantedRejected d (projectionNames schema) seen n wire consumed offset left reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       globalAddr (projectionGlobal schema) ↦□ s.fields' ∗ FeltErrorGlobals kinds ∗ c ↦ v ∗ st ↦ s ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ {v with pos' := v.pos' + W64 consumed} ∗ st ↦ {s with left' := W64 left} ∗ v.buf' ↦*{dq} xs ∗
       WantedFailureError err (v.pos' + W64 offset) reason (wantedFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hglobal, Hglobals, Hc, Hst, Hs⟩
  ihave #Htable := projection_initialized_table (GF := GF) schema $$ []
  · solve_pkg_init
  isimp only [ProjectionTable] at Htable
  icases Htable with ⟨%fields, #Hpublished, #Htable⟩
  ihave %agree := typedPointsto_agree_wand (globalAddr (projectionGlobal schema))
    DFrac.discard DFrac.discard fields s.fields' $$ Hpublished Hglobal
  subst fields
  isimp only [StaticFieldsShared] at Htable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %Hmask⟩
  wp_apply wp_nextWanted_rejected validator c st s.fields' v s f (projectionNames schema) seen xs wire
    DFrac.discard DFrac.discard dq d n consumed offset left reason kinds rfl hleft countBound
    (projectionNames_small schema) (projectionNames_distinct schema) hs hd depth failure hp hbytes
    $$ [Hglobals Hc Hst Hs] as %err ⟨Hc, Hst, _, _, Hs, Herr, %Hnext⟩
  · iframe Hglobals Hc Hst Hfields Hnames Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_projection_rejected
