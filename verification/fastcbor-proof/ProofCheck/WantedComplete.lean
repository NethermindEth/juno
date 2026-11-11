import ProofCheck.WantedRejectedImpl
import ProofCheck.WantedFailureUnique
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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Concrete state and diagnostic ownership for the independent scan outcome.
    Successful scans leave the selected value unread or close an exhausted map.
    Rejection retains the failing pair and the original container-open state. -/
def WantedScanOwned (c st : Loc) (v : cursor.Cursor) (s : cursor.Struct)
    (xs wire : List w8) (kinds : FeltErrorKinds) (code : w64) (result : GoInterface) :
    WantedOutcome → IProp GF
  | .success selected rest => iprop(∃ item : List w8,
      c ↦ wantedCursor v s.open' (v.pos' + W64 item.length) selected ∗
      st ↦ wantedStruct s selected ∗
      ⌜code = wantedCode selected ∧ result = interface.nil ∧ wire = item ++ rest ∧
        sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
        sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure consumed offset left reason => iprop(∃ err : Loc,
      c ↦ {v with pos' := v.pos' + W64 consumed} ∗ st ↦ {s with left' := W64 left} ∗
      WantedFailureError err (v.pos' + W64 offset) reason (wantedFailureKind reason kinds) ∗
      ⌜code = W64 (-1) ∧ result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

/-- Actual NextWanted on every byte input at a valid cursor, with metadata and
    seen-mask correspondence. The input need not be a valid CBOR map suffix.
    Initialized cursor globals supply actual error categories; the UTF-8 helper
    remains the explicit shared implementation premise. -/
theorem wp_nextWanted_complete (validator : Utf8ValidatorContract (GF := GF))
    (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (seen : List Nat) (xs wire : List w8) (qf qn dq : DFrac)
    (d n : Nat)
    (hfields : s.fields' = fields) (hleft : s.left' = W64 n) (countBound : n < 2^63)
    (small : names.length ≤ 64) (distinct : (names.map (List.map fieldFoldByte)).Nodup)
    (allMask : f.all' = (W64 1 <<< W64 names.length) - W64 1)
    (hs : WantedSeen names.length s.seen' seen)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ (code : w64) (result : GoInterface), RET (PairV #code #result);
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ∃ (outcome : WantedOutcome) (kinds : FeltErrorKinds),
         FeltErrorGlobals kinds ∗ ⌜WantedSpec d n names seen wire outcome⌝ ∗
         WantedScanOwned c st v s xs wire kinds code result outcome }} := by
  wp_start_folded as ⟨Hc, Hst, Hfields, Hnames, Hs⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply cursor_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  rcases wanted_classify d n names seen wire depth with
    ⟨item, rest, selected, split, valid⟩ | ⟨consumed, offset, left, reason, failure⟩
  · wp_apply wp_nextWanted_valid validator c st fields v s f names seen xs item rest qf qn dq d n selected
      hfields hleft countBound small distinct allMask hs hd depth valid hp (hbytes.trans split)
      $$ [Hc Hst Hfields Hnames Hs] with ⟨Hc, Hst, Hfields, Hnames, Hs, %Hnext⟩
    · iframe Hc Hst Hfields Hnames Hs
    iapply HΦ
    iframe Hfields Hnames Hs
    iexists (WantedOutcome.success selected rest), kinds
    iframe Hglobals
    isplitr
    · ipureintro; exact ⟨item, split, valid⟩
    isimp only [WantedScanOwned]
    iexists item
    iframe Hc Hst
    ipureintro; exact ⟨trivial, trivial, split, Hnext⟩
  · wp_apply wp_nextWanted_rejected validator c st fields v s f names seen xs wire qf qn dq
      d n consumed offset left reason kinds hfields hleft countBound small distinct hs hd depth failure hp hbytes
      $$ [Hc Hst Hfields Hnames Hs] as %err ⟨Hc, Hst, Hfields, Hnames, Hs, Herr, %Hnext⟩
    · iframe Hglobals Hc Hst Hfields Hnames Hs
    iapply HΦ
    iframe Hfields Hnames Hs
    iexists (WantedOutcome.failure consumed offset left reason), kinds
    iframe Hglobals
    isplitr
    · ipureintro; exact failure
    isimp only [WantedScanOwned]
    iexists err
    iframe Hc Hst Herr
    ipureintro; exact ⟨trivial, rfl, Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_complete
