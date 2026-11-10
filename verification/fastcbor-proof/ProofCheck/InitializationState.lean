import ProofCheck.SchemaState

import Perennial.Proof.encoding.binary.Base
import ProofCheck.GeneratedProof.math.big
import Perennial.Proof.strings.Base
import Perennial.Proof.slices.Base
import Perennial.Proof.unsafe.Base
import Perennial.Proof.errors.NewVar
import Perennial.Proof.fmt.Base

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp]
variable [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

instance depInit0 : IsPkgInit (IProp GF) pkg_id.github_com.consensys.gnark_crypto.ecc.stark_curve.fp :=
  define_is_pkg_init iprop(True)
instance depWf0 : GetIsPkgInitWf (IProp GF) pkg_id.github_com.consensys.gnark_crypto.ecc.stark_curve.fp :=
  build_get_is_pkg_init_wf

instance depInit1 : IsPkgInit (IProp GF) pkg_id.github_com.NethermindEth.juno.core.felt :=
  define_is_pkg_init iprop(True)
instance depWf1 : GetIsPkgInitWf (IProp GF) pkg_id.github_com.NethermindEth.juno.core.felt :=
  build_get_is_pkg_init_wf

/-- First-byte classes used by Go's UTF-8 validator: low bits are the sequence
    length and high bits select the permitted second-byte range. -/
def utf8FirstTable : GoArray w8 256 :=
  array.mk 256 ((List.range 256).map fun b => W8 (
    if b < 128 then 240 else if b < 194 then 241 else if b < 224 then 2
    else if b = 224 then 19 else if b < 237 then 3 else if b = 237 then 35
    else if b < 240 then 3 else if b = 240 then 52 else if b < 244 then 4
    else if b = 244 then 68 else 241))

/-- The second-byte restrictions exclude overlong forms, surrogates and values
    above U+10FFFF. Unused entries remain zero, as in the Go array literal. -/
def utf8AcceptTable : GoArray unicode.utf8.acceptRange 16 :=
  array.mk 16 ([⟨W8 128, W8 191⟩, ⟨W8 160, W8 191⟩, ⟨W8 128, W8 159⟩,
    ⟨W8 144, W8 191⟩, ⟨W8 128, W8 143⟩] ++ List.replicate 11 ⟨W8 0, W8 0⟩)

/-- Actual table contents retained after UTF-8 package startup. -/
def Utf8Initialized : IProp GF := iprop(
  globalAddr unicode.utf8.first ↦□ utf8FirstTable ∗
  globalAddr unicode.utf8.acceptRanges ↦□ utf8AcceptTable)

instance utf8Initialized_persistent : Persistent (Utf8Initialized (GF := GF)) := by
  unfold Utf8Initialized
  infer_instance

instance depInit2 : IsPkgInit (IProp GF) pkg_id.unicode.utf8 :=
  define_is_pkg_init (Utf8Initialized (GF := GF))
instance depWf2 : GetIsPkgInitWf (IProp GF) pkg_id.unicode.utf8 :=
  build_get_is_pkg_init_wf

/-- Persistent evidence that a package completed startup in the modeled Go state.
    Runtime CPU startup is an explicit precondition, not an initializer axiom. -/
def PackageInitialized (pkg : GoString) : IProp GF :=
  iprop(∃ σ0 : GMap GoString Bool, ownGoStateLb σ0 ∗ ⌜σ0 !! pkg = some true⌝)

instance packageInitialized_persistent (pkg : GoString) :
    Persistent (PackageInitialized (GF := GF) pkg) := by
  unfold PackageInitialized
  infer_instance

/-- The two immutable runtime-populated feature flags read by math/big startup.
    Values are arbitrary: the proof does not assume a particular CPU capability. -/
def RuntimeCPUFlags : IProp GF := iprop(∃ adx bmi2 : Bool,
  (globalAddr internal.cpu.X86).[internal.cpu.«struct#08e4e4b9», go!"HasADX"] ↦□ adx ∗
  (globalAddr internal.cpu.X86).[internal.cpu.«struct#08e4e4b9», go!"HasBMI2"] ↦□ bmi2)

instance runtimeCPUFlags_persistent : Persistent (RuntimeCPUFlags (GF := GF)) := by
  unfold RuntimeCPUFlags
  infer_instance

instance cpuInit : IsPkgInit (IProp GF) pkg_id.internal.cpu :=
  define_is_pkg_init (RuntimeCPUFlags (GF := GF))
instance cpuWf : GetIsPkgInitWf (IProp GF) pkg_id.internal.cpu := build_get_is_pkg_init_wf

instance depInitBig : IsPkgInit (IProp GF) pkg_id.math.big :=
  define_is_pkg_init iprop(True)
instance depWfBig : GetIsPkgInitWf (IProp GF) pkg_id.math.big :=
  build_get_is_pkg_init_wf

/-- The actual immutable error objects retained by cursor package initialization. -/
def CursorInitialized : IProp GF := iprop(
  errors.newVar cursor.ErrTruncated go!"cbor: unexpected end of input" ∗
  errors.newVar cursor.ErrSyntax go!"cbor: malformed item" ∗
  errors.newVar cursor.ErrType go!"cbor: item does not fit the destination" ∗
  errors.newVar cursor.ErrDepth go!"cbor: nesting too deep" ∗
  errors.newVar cursor.ErrUnsupported go!"cbor: unsupported encoding")

instance cursorInitialized_persistent : Persistent (CursorInitialized (GF := GF)) := by
  unfold CursorInitialized
  infer_instance

instance depInit3 : IsPkgInit (IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor :=
  define_is_pkg_init (CursorInitialized (GF := GF))
instance depWf3 : GetIsPkgInitWf (IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor :=
  build_get_is_pkg_init_wf

instance depInit4 : IsPkgInit (IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.fastcbor :=
  define_is_pkg_init iprop(True)
instance depWf4 : GetIsPkgInitWf (IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.fastcbor :=
  build_get_is_pkg_init_wf

instance schemaIsPkgInit : IsPkgInit (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema :=
  define_is_pkg_init (HeaderMetadata (GF := GF))
instance schemaGetIsPkgInitWf : GetIsPkgInitWf (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema :=
  build_get_is_pkg_init_wf

instance coreIsPkgInit : IsPkgInit (IProp GF) pkg_id.github_com.NethermindEth.juno.core :=
  define_is_pkg_init iprop(True)
instance coreGetIsPkgInitWf : GetIsPkgInitWf (IProp GF) pkg_id.github_com.NethermindEth.juno.core :=
  build_get_is_pkg_init_wf

/-- Standard modular initialization contract. Its predicate records the actual
    resources established by the package, including those of its dependencies. -/
abbrev DecoderDependencyInit (pkg : GoString) (initFn : val)
    [IsPkgInit (IProp GF) pkg] [GetIsPkgInitWf (IProp GF) pkg] : Prop :=
  ∀ get_is_pkg_init : GoString → IProp GF,
    GetIsPkgInitProp pkg get_is_pkg_init →
    {{ ownInitializing get_is_pkg_init }}
      (App (Val initFn) (Val #()))
    {{ RET #(); ownInitializing get_is_pkg_init ∗ isPkgInit (PROP := IProp GF) pkg }}

/-- Obtain the error objects guaranteed by the cursor initialization contract. -/
theorem cursor_initialized_errors :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ⊢
      CursorInitialized (GF := GF) := by
  exact isPkgInit_access _

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.cursor_initialized_errors
