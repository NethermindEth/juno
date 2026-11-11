import ProofCheck.SegmentKeys
import ProofCheck.StructOpenSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def SegmentChoiceFresh (seen : List SegmentField) : Option SegmentField → Prop
  | none => True
  | some field => field ∉ seen

def segmentChoiceSeen (seen : List SegmentField) : Option SegmentField → List SegmentField
  | none => seen
  | some field => field :: seen

/- Recursive record/array relations preserve the actual later bytes for count
    guards. Record and array openings each consume one level of nesting budget. -/
mutual
inductive SegmentFieldValid : Nat → SegmentField → SegmentValue → List w8 → List w8 → SegmentValue → Prop where
  | children {d : Nat} {old : SegmentValue} {item rest value}
      (valid : SegmentArraySpec d (item ++ rest) (.success value rest)) :
      SegmentFieldValid d .children old item rest (segmentSetChildren old value)
  | length {d : Nat} {old : SegmentValue} {item rest value} (valid : ScalarValid old.length item value) :
      SegmentFieldValid d .length old item rest (segmentSetLength old value)

inductive SegmentFieldRejected : Nat → SegmentField → List w8 → Nat → Nat → Nat → SegmentFieldFailure → Prop where
  | children {d : Nat} {wire consumed offset extraDepth reason}
      (failure : SegmentArraySpec d wire (.failure consumed offset extraDepth reason)) :
      SegmentFieldRejected d .children wire consumed offset extraDepth (.children reason)
  | length {d : Nat} {wire reason} (failure : ScalarFailure wire reason) :
      SegmentFieldRejected d .length wire 0 0 0 (.scalar reason)

inductive SegmentValueValid : Nat → Option SegmentField → SegmentValue → List w8 → List w8 → SegmentValue → Prop where
  | selected {d : Nat} {field old item rest final} (valid : SegmentFieldValid d field old item rest final) :
      SegmentValueValid d (some field) old item rest final
  | ignored {d : Nat} {old : SegmentValue} {item rest} (valid : SkipValid d item) :
      SegmentValueValid d none old item rest old

inductive SegmentValueRejected : Nat → Option SegmentField → SegmentValue → List w8 → SegmentValue →
    Nat → Nat → Nat → SegmentValueFailure → Prop where
  | selected {d : Nat} {field old wire consumed offset extraDepth reason}
      (failure : SegmentFieldRejected d field wire consumed offset extraDepth reason) :
      SegmentValueRejected d (some field) old wire old consumed offset extraDepth (.field reason)
  | ignored {d : Nat} {old : SegmentValue} {wire offset reason} (failure : SkipFailure d wire offset reason) :
      SegmentValueRejected d none old wire old 0 offset 0 (.ignored reason)

inductive SegmentPairsValid : Nat → Nat → List SegmentField → SegmentValue → List w8 → List w8 → SegmentValue → Prop where
  | nil {d : Nat} (seen : List SegmentField) (old : SegmentValue) (suffix : List w8) :
      SegmentPairsValid d 0 seen old [] suffix old
  | step {d : Nat} {n seen choice old middle final key value tail suffix}
      (keyValid : SegmentKeyValid key choice) (fresh : SegmentChoiceFresh seen choice)
      (valueValid : SegmentValueValid d choice old value (tail ++ suffix) middle)
      (rest : SegmentPairsValid d n (segmentChoiceSeen seen choice) middle tail suffix final) :
      SegmentPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

inductive SegmentPairsRejected : Nat → Nat → List SegmentField → SegmentValue → List w8 → SegmentValue →
    Nat → Nat → Nat → SegmentFailureReason → Prop where
  | key {d : Nat} {n seen old wire reason} (failure : KeyRejected wire reason) :
      SegmentPairsRejected d (n + 1) seen old wire old 0 0 0 (.key reason)
  | duplicate {d : Nat} {n seen old key tail field} (valid : SegmentKeyValid key (some field)) (duplicate : field ∈ seen) :
      SegmentPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length 0 (.duplicate field)
  | value {d : Nat} {n seen choice old final key tail consumed offset extraDepth reason} (keyValid : SegmentKeyValid key choice)
      (fresh : SegmentChoiceFresh seen choice)
      (failure : SegmentValueRejected d choice old tail final consumed offset extraDepth reason) :
      SegmentPairsRejected d (n + 1) seen old (key ++ tail) final
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {d : Nat} {n seen choice old middle final key value tail consumed offset extraDepth reason}
      (keyValid : SegmentKeyValid key choice) (fresh : SegmentChoiceFresh seen choice)
      (valueValid : SegmentValueValid d choice old value tail middle)
      (failure : SegmentPairsRejected d n (segmentChoiceSeen seen choice) middle tail final consumed offset extraDepth reason) :
      SegmentPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

inductive SegmentRecordValid : Nat → SegmentValue → List w8 → List w8 → SegmentValue → Prop where
  | nullish {d : Nat} {old : SegmentValue} {head : w8} (suffix : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      SegmentRecordValid d old [head] suffix old
  | map {d : Nat} {old : SegmentValue} {head payload body suffix argument final} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (depth : d < 32)
      (valid : SegmentPairsValid (d + 1) count [] old body suffix final) :
      SegmentRecordValid d old ((head :: payload) ++ body) suffix final

inductive SegmentRecordRejected : Nat → SegmentValue → List w8 → SegmentValue →
    Nat → Nat → Nat → SegmentFailureReason → Prop where
  | opening {d : Nat} {old : SegmentValue} {wire reason} (failure : StructOpenFailure d wire reason) :
      SegmentRecordRejected d old wire old 0 0 0 (.opening reason)
  | map {d : Nat} {old : SegmentValue} {head payload body argument final consumed offset extraDepth reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (depth : d < 32) (fit : 2 * count ≤ body.length)
      (failure : SegmentPairsRejected (d + 1) count [] old body final consumed offset extraDepth reason) :
      SegmentRecordRejected d old ((head :: payload) ++ body) final
        (1 + payload.length + consumed) (1 + payload.length + offset) (1 + extraDepth) reason

inductive SegmentItemsValid : Nat → Nat → List w8 → List w8 → List SegmentValue → Prop where
  | nil {d : Nat} (suffix : List w8) : SegmentItemsValid d 0 [] suffix []
  | step {d : Nat} {n item tail suffix value values}
      (valid : SegmentRecordValid d segmentZero item (tail ++ suffix) value)
      (rest : SegmentItemsValid d n tail suffix values) :
      SegmentItemsValid d (n + 1) (item ++ tail) suffix (value :: values)

inductive SegmentItemsRejected : Nat → Nat → List w8 → List SegmentValue → SegmentValue →
    Nat → Nat → Nat → SegmentFailureReason → Prop where
  | here {d : Nat} {n wire retained consumed offset childDepth reason}
      (failure : SegmentRecordRejected d segmentZero wire retained consumed offset childDepth reason) :
      SegmentItemsRejected d (n + 1) wire [] retained consumed offset childDepth reason
  | next {d : Nat} {n item tail value done retained consumed offset childDepth reason}
      (valid : SegmentRecordValid d segmentZero item tail value)
      (failure : SegmentItemsRejected d n tail done retained consumed offset childDepth reason) :
      SegmentItemsRejected d (n + 1) (item ++ tail) (value :: done) retained
        (item.length + consumed) (item.length + offset) childDepth reason

inductive SegmentArraySpec : Nat → List w8 → SegmentArrayOutcome → Prop where
  | nullish {d : Nat} (head : w8) (tail : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      SegmentArraySpec d (head :: tail) (.success none tail)
  | outer {d : Nat} {wire reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 128) wire reason) :
      SegmentArraySpec d wire (.failure 0 0 0 (.expected reason))
  | count {d : Nat} {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (short : tail.length < count) :
      SegmentArraySpec d ((head :: payload) ++ tail) (.failure 0 0 0 .count)
  | depth {d : Nat} {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ tail.length) (deep : 32 ≤ d) :
      SegmentArraySpec d ((head :: payload) ++ tail) (.failure 0 0 0 .depth)
  | array {d : Nat} {head payload item rest argument values} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ (item ++ rest).length) (depth : d < 32)
      (valid : SegmentItemsValid (d + 1) count item rest values) :
      SegmentArraySpec d ((head :: payload) ++ item ++ rest) (.success (some values) rest)
  | element {d : Nat} {head payload tail argument done retained consumed offset childDepth reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ tail.length) (depth : d < 32)
      (failure : SegmentItemsRejected (d + 1) count tail done retained consumed offset childDepth reason) :
      SegmentArraySpec d ((head :: payload) ++ tail)
        (.failure (1 + payload.length + consumed) (1 + payload.length + offset)
          (1 + childDepth) (.element reason))

end

end github_com.NethermindEth.juno.core
end Perennial
