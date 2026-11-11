import ProofCheck.IndexesOwnership
import ProofCheck.IntegerKeyComplete

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

def indexesSkipKinds (k : BigIntErrorKinds) (depthKind : Loc) : SkipErrorKinds :=
  ⟨bigIntCategory k.syntaxError, bigIntCategory k.truncated, bigIntCategory k.unsupported, bigIntCategory depthKind⟩

def indexesFeltKinds (k : BigIntErrorKinds) (depthKind : Loc) : FeltErrorKinds :=
  ⟨indexesSkipKinds k depthKind, bigIntCategory k.typeError⟩

/-- Concrete returned errors retain the bare unsigned-overflow sentinel versus
    owned diagnostic distinction, across keys, arrays and ignored values. -/
def IndexesRecordError (result : GoInterface) (pos : w64) (reason : IndexesRecordFailureReason)
    (k : BigIntErrorKinds) (depthKind : Loc) : IProp GF :=
  match reason with
  | .opening reason => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      FeltFailureError err pos reason (feltFailureKind reason (indexesFeltKinds k depthKind)))
  | .key reason => IntegerKeyError result pos reason k
  | .value (.selected reason) => IntsFailureError result pos reason k depthKind
  | .value (.ignored reason) => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      SkipFailureError err pos reason (skipFailureKind reason (indexesSkipKinds k depthKind)))

/-- Both concrete error representations are nonnil Go interface values. -/
def IndexesErrorShape (result : GoInterface) (k : BigIntErrorKinds) : Prop :=
  result = bigIntCategory k.typeError ∨ ∃ err : Loc, result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err

theorem integerKeyError_shape (result : GoInterface) (pos : w64) (reason : IntegerKeyFailureReason) (k : BigIntErrorKinds) :
    IntegerKeyError (GF := GF) result pos reason k ⊢ ⌜IndexesErrorShape result k⌝ := by
  cases reason with
  | overflow =>
    unfold IntegerKeyError
    iintro %h
    ipureintro
    exact Or.inl h
  | unsigned _ | other _ =>
    unfold IntegerKeyError
    iintro ⟨%err, %h, _⟩
    ipureintro
    exact Or.inr ⟨err, h⟩

theorem intsFailureError_shape (result : GoInterface) (pos : w64) (reason : IntsFailureReason)
    (k : BigIntErrorKinds) (depthKind : Loc) :
    IntsFailureError (GF := GF) result pos reason k depthKind ⊢ ⌜IndexesErrorShape result k⌝ := by
  cases reason with
  | outer _ | count =>
    unfold IntsFailureError
    iintro ⟨%err, %h, _⟩
    ipureintro
    exact Or.inr ⟨err, h⟩
  | depth =>
    unfold IntsFailureError
    iintro ⟨%err, %args, %h, _, _⟩
    ipureintro
    exact Or.inr ⟨err, h⟩
  | element reason =>
    cases reason with
    | unsignedOverflow =>
      unfold IntsFailureError IntElementFailureError
      iintro %h
      ipureintro
      exact Or.inl h
    | signed _ =>
      unfold IntsFailureError IntElementFailureError
      iintro ⟨%err, %h, _⟩
      ipureintro
      exact Or.inr ⟨err, h⟩

theorem indexesSkip_globals (k : BigIntErrorKinds) (depthKind : Loc) :
    BigIntErrorGlobals k ∗ globalAddr cursor.ErrDepth ↦□ bigIntCategory depthKind ⊢
      (SkipErrorGlobals (indexesSkipKinds k depthKind) : IProp GF) := by
  unfold BigIntErrorGlobals SkipErrorGlobals indexesSkipKinds
  iintro ⟨⟨#Ht, #Hs, #Htype, #Hu, _⟩, #Hd⟩
  iframe Hs Ht Hu Hd

theorem indexesFelt_globals (k : BigIntErrorKinds) (depthKind : Loc) :
    BigIntErrorGlobals k ∗ globalAddr cursor.ErrDepth ↦□ bigIntCategory depthKind ⊢
      (FeltErrorGlobals (indexesFeltKinds k depthKind) : IProp GF) := by
  unfold BigIntErrorGlobals FeltErrorGlobals SkipErrorGlobals indexesFeltKinds indexesSkipKinds
  iintro ⟨⟨#Ht, #Hs, #Htype, #Hu, _⟩, #Hd⟩
  iframe Hs Ht Hu Hd Htype

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.integerKeyError_shape
#print axioms Perennial.github_com.NethermindEth.juno.core.intsFailureError_shape
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesSkip_globals

#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFelt_globals
