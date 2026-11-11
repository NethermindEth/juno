import ProofCheck.ScalarComplete
import ProofCheck.KeySpec
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A finite stored segment tree. Nil children differ from an allocated empty
    list. Length is decoded storage; no sum-of-children invariant is imposed. -/
inductive SegmentValue where
  | mk (children : Option (List SegmentValue)) (length : w64)

def SegmentValue.children : SegmentValue → Option (List SegmentValue)
  | .mk children _ => children

def SegmentValue.length : SegmentValue → w64
  | .mk _ length => length

def segmentZero : SegmentValue := .mk none (W64 0)
def segmentSetChildren (old : SegmentValue) (children : Option (List SegmentValue)) : SegmentValue :=
  .mk children old.length

def segmentSetLength (old : SegmentValue) (length : w64) : SegmentValue := .mk old.children length

inductive SegmentField where
  | children | length
  deriving DecidableEq

def segmentFields : List SegmentField := [.children, .length]
def segmentFieldName : SegmentField → GoString
  | .children => go!"Children"
  | .length => go!"Length"
def segmentNames : List GoString := [go!"Children", go!"Length"]

theorem segmentField_names : segmentFields.map segmentFieldName = segmentNames := rfl

theorem segmentValue_eta (old : SegmentValue) : SegmentValue.mk old.children old.length = old := by cases old; rfl

theorem segmentSetChildren_preserves_length (old : SegmentValue) (children : Option (List SegmentValue)) :
    (segmentSetChildren old children).length = old.length := rfl

theorem segmentSetLength_preserves_children (old : SegmentValue) (length : w64) :
    (segmentSetLength old length).children = old.children := rfl

/- Diagnostic paths are recursive too: an array element can fail in a record
    whose Children array contains another failed record. -/
mutual
inductive SegmentFieldFailure where
  | children (reason : SegmentArrayFailureReason)
  | scalar (reason : ExpectedFailureReason)
inductive SegmentValueFailure where
  | field (reason : SegmentFieldFailure)
  | ignored (reason : SkipFailureReason)
inductive SegmentFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : SegmentField)
  | value (reason : SegmentValueFailure)
inductive SegmentArrayFailureReason where
  | expected (reason : ExpectedFailureReason)
  | count
  | depth
  | element (reason : SegmentFailureReason)
end

inductive SegmentArrayOutcome where
  | success (values : Option (List SegmentValue)) (rest : List w8)
  | failure (consumed offset extraDepth : Nat) (reason : SegmentArrayFailureReason)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentValue_eta
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentSetChildren_preserves_length
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentSetLength_preserves_children
