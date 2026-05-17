package validate_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/wuwuseo/cmf/validate"
)

// ======================== 测试用结构体 ========================

// TestUser 基础测试结构体
type TestUser struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
	Age   int    `validate:"min=18,max=120"`
}

// TestUserWithMessages 实现 MessageProvider 接口的结构体
type TestUserWithMessages struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
}

func (t TestUserWithMessages) GetMessagesMap() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"Name.required":  "用户名不能为空",
		"Email.required": "邮箱不能为空",
		"Email.email":    "邮箱格式不正确",
	}
}

// TestUserOnlyRequired 仅有必填字段的结构体
type TestUserOnlyRequired struct {
	Name string `validate:"required"`
}

// TestTagRequired required 标签测试
type TestTagRequired struct {
	Name string `validate:"required"`
}

// TestTagEmail email 标签测试
type TestTagEmail struct {
	Email string `validate:"email"`
}

// TestTagURL url 标签测试
type TestTagURL struct {
	URL string `validate:"url"`
}

// TestTagMinString 字符串 min 标签测试
type TestTagMinString struct {
	Name string `validate:"min=3"`
}

// TestTagMinInt 整数 min 标签测试
type TestTagMinInt struct {
	Age int `validate:"min=18"`
}

// TestTagMaxString 字符串 max 标签测试
type TestTagMaxString struct {
	Name string `validate:"max=10"`
}

// TestTagMaxInt 整数 max 标签测试
type TestTagMaxInt struct {
	Age int `validate:"max=120"`
}

// TestTagLen len 标签测试
type TestTagLen struct {
	Code string `validate:"len=6"`
}

// TestTagOneof oneof 标签测试
type TestTagOneof struct {
	Status string `validate:"oneof=active inactive"`
}

// TestTagIP ip 标签测试
type TestTagIP struct {
	IP string `validate:"ip"`
}

// TestTagUUID uuid 标签测试
type TestTagUUID struct {
	ID string `validate:"uuid"`
}

// TestTagAlpha alpha 标签测试
type TestTagAlpha struct {
	Name string `validate:"alpha"`
}

// TestTagNumeric numeric 标签测试
type TestTagNumeric struct {
	Value string `validate:"numeric"`
}

// TestCustomTag 自定义验证标签测试
type TestCustomTag struct {
	Code string `validate:"custom_even"`
}

// ======================== 1. 新验证器 ========================

func TestNewValidator(t *testing.T) {
	v := validate.NewValidator()
	if v == nil {
		t.Fatal("NewValidator 应返回非 nil 的验证器实例")
	}
}

func TestValidator_Instance(t *testing.T) {
	v := validate.NewValidator()
	inst := v.Instance()
	if inst == nil {
		t.Fatal("Instance 应返回非 nil 的 validator.Validate 实例")
	}

	// 验证返回的实例可用
	type Simple struct {
		Name string `validate:"required"`
	}
	err := inst.Struct(Simple{Name: "test"})
	if err != nil {
		t.Fatalf("底层实例验证应通过，但返回了错误: %v", err)
	}
}

// ======================== 2. 结构体验证 ========================

func TestValidator_ValidateStruct_Success(t *testing.T) {
	v := validate.NewValidator()
	user := TestUser{
		Name:  "张三",
		Email: "zhangsan@example.com",
		Age:   25,
	}
	err := v.ValidateStruct(user)
	if err != nil {
		t.Fatalf("有效结构体验证应通过，但返回了错误: %v", err)
	}
}

func TestValidator_ValidateStruct_Required(t *testing.T) {
	v := validate.NewValidator()
	user := TestUser{}
	err := v.ValidateStruct(user)
	if err == nil {
		t.Fatal("缺少必填字段应返回验证错误")
	}
}

func TestValidator_ValidateStruct_Email(t *testing.T) {
	v := validate.NewValidator()
	user := TestUser{
		Name:  "张三",
		Email: "invalid-email",
		Age:   25,
	}
	err := v.ValidateStruct(user)
	if err == nil {
		t.Fatal("邮箱格式错误应返回验证错误")
	}
}

func TestValidator_ValidateStruct_MinMax(t *testing.T) {
	v := validate.NewValidator()

	// 测试 min 小于最小值
	user := TestUser{
		Name:  "张三",
		Email: "zhangsan@example.com",
		Age:   10,
	}
	err := v.ValidateStruct(user)
	if err == nil {
		t.Fatal("Age 小于 18 应返回验证错误")
	}

	// 测试 max 大于最大值
	user.Age = 150
	err = v.ValidateStruct(user)
	if err == nil {
		t.Fatal("Age 大于 120 应返回验证错误")
	}
}

func TestValidator_RegisterValidation(t *testing.T) {
	v := validate.NewValidator()

	// 注册自定义规则：值必须是偶数
	err := v.RegisterValidation("custom_even", func(fl validator.FieldLevel) bool {
		val := fl.Field().Int()
		return val%2 == 0
	})
	if err != nil {
		t.Fatalf("注册自定义验证规则应成功，但返回了错误: %v", err)
	}

	// 验证有效值（偶数）
	valid := TestCustomTag{Code: "2"} // 注意：code 是字符串字段，这里适配一下
	_ = valid
	err = v.ValidateStruct(struct {
		Value int `validate:"custom_even"`
	}{Value: 4})
	if err != nil {
		t.Fatalf("偶数应通过自定义验证，但返回了错误: %v", err)
	}

	// 验证无效值（奇数）
	err = v.ValidateStruct(struct {
		Value int `validate:"custom_even"`
	}{Value: 3})
	if err == nil {
		t.Fatal("奇数应不通过自定义验证")
	}
}

func TestValidator_SetDefaultMessage(t *testing.T) {
	v := validate.NewValidator()

	// 设置自定义默认消息，验证不会 panic
	v.SetDefaultMessage("required", "此字段为必填项")
	v.SetDefaultMessage("email", "请输入正确的邮箱地址")

	// 设置后，验证器应仍可正常使用
	user := TestUser{
		Name:  "张三",
		Email: "zhangsan@example.com",
		Age:   25,
	}
	err := v.ValidateStruct(user)
	if err != nil {
		t.Fatalf("设置默认消息后验证应正常通过，但返回了错误: %v", err)
	}
}

// ======================== 3. 错误格式化 ========================

func TestFormatValidationErrors(t *testing.T) {
	v := validate.NewValidator()
	user := TestUserOnlyRequired{}
	err := v.ValidateStruct(user)
	if err == nil {
		t.Fatal("应返回验证错误")
	}

	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("验证错误应被格式化，但返回了空切片")
	}

	// 检查错误字段
	found := false
	for _, e := range errs {
		if e.Field == "Name" && e.Tag == "required" {
			found = true
			if e.Message != "Name为必填字段" {
				t.Errorf("required 错误消息不正确: %s", e.Message)
			}
			break
		}
	}
	if !found {
		t.Error("未找到 Name 字段的 required 验证错误")
	}
}

func TestFormatValidationErrors_NonValidationError(t *testing.T) {
	// 传入非验证错误，应返回 nil
	plainErr := errors.New("这是一个普通错误")
	errs := validate.FormatValidationErrors(plainErr)
	if errs != nil {
		t.Error("非验证错误应返回 nil")
	}
}

func TestFormatValidationErrorsWithInput(t *testing.T) {
	v := validate.NewValidator()
	input := TestUserWithMessages{}
	err := v.ValidateStruct(input)
	if err == nil {
		t.Fatal("应返回验证错误")
	}

	errs := validate.FormatValidationErrorsWithInput(input, err)
	if len(errs) == 0 {
		t.Fatal("验证错误应被格式化，但返回了空切片")
	}

	// 应使用自定义消息
	for _, e := range errs {
		switch {
		case e.Field == "Name" && e.Tag == "required":
			if e.Message != "用户名不能为空" {
				t.Errorf("Name.required 应使用自定义消息'用户名不能为空'，实际: %s", e.Message)
			}
		case e.Field == "Email" && e.Tag == "required":
			if e.Message != "邮箱不能为空" {
				t.Errorf("Email.required 应使用自定义消息'邮箱不能为空'，实际: %s", e.Message)
			}
		case e.Field == "Email" && e.Tag == "email":
			if e.Message != "邮箱格式不正确" {
				t.Errorf("Email.email 应使用自定义消息'邮箱格式不正确'，实际: %s", e.Message)
			}
		}
	}
}

func TestFormatValidationErrorsWithInput_NoMessageProvider(t *testing.T) {
	v := validate.NewValidator()
	// TestUserOnlyRequired 未实现 MessageProvider
	input := TestUserOnlyRequired{}
	err := v.ValidateStruct(input)
	if err == nil {
		t.Fatal("应返回验证错误")
	}

	errs := validate.FormatValidationErrorsWithInput(input, err)
	if len(errs) == 0 {
		t.Fatal("验证错误应被格式化，但返回了空切片")
	}

	// 应使用默认中文消息
	for _, e := range errs {
		if e.Field == "Name" && e.Tag == "required" {
			if e.Message != "Name为必填字段" {
				t.Errorf("未实现 MessageProvider 时应使用默认消息，实际: %s", e.Message)
			}
		}
	}
}

// ======================== 4. GetValidatorErrorMsg ========================

func TestGetValidatorErrorMsg(t *testing.T) {
	v := validate.NewValidator()
	input := TestUserWithMessages{}
	err := v.ValidateStruct(input)
	if err == nil {
		t.Fatal("应返回验证错误")
	}

	msg := validate.GetValidatorErrorMsg(input, err)

	// 错误消息应以逗号分隔
	if !strings.Contains(msg, "用户名不能为空") {
		t.Errorf("错误消息应包含'用户名不能为空'，实际: %s", msg)
	}
	if !strings.Contains(msg, "邮箱不能为空") {
		t.Errorf("错误消息应包含'邮箱不能为空'，实际: %s", msg)
	}

	// 消息应以逗号分隔
	if !strings.Contains(msg, ",") {
		t.Errorf("多条错误消息应以逗号分隔，实际: %s", msg)
	}
}

func TestGetValidatorErrorMsg_NoError(t *testing.T) {
	// 传入 nil 错误，应返回默认消息
	msg := validate.GetValidatorErrorMsg(nil, nil)
	if msg != "Parameter error" {
		t.Errorf("无错误时应返回'Parameter error'，实际: %s", msg)
	}

	// 传入普通非验证错误
	plainErr := errors.New("普通错误")
	msg = validate.GetValidatorErrorMsg(nil, plainErr)
	if msg != "Parameter error" {
		t.Errorf("非验证错误时应返回'Parameter error'，实际: %s", msg)
	}
}

// ======================== 5. MessageProvider 接口 ========================

func TestMessageProvider(t *testing.T) {
	v := validate.NewValidator()
	input := TestUserWithMessages{
		Name:  "",
		Email: "",
	}
	err := v.ValidateStruct(input)
	if err == nil {
		t.Fatal("应返回验证错误")
	}

	errs := validate.FormatValidationErrorsWithInput(input, err)

	expectedMessages := map[string]string{
		"Name.required":  "用户名不能为空",
		"Email.required": "邮箱不能为空",
	}

	for _, e := range errs {
		key := e.Field + "." + e.Tag
		if expected, ok := expectedMessages[key]; ok {
			if e.Message != expected {
				t.Errorf("键 %s 应使用自定义消息'%s'，实际: %s", key, expected, e.Message)
			}
		}
	}
}

// ======================== 6. 全局 Validate ========================

func TestGlobalValidate(t *testing.T) {
	if validate.Validate == nil {
		t.Fatal("全局 Validate 不应为 nil")
	}
}

func TestGlobalValidate_Struct(t *testing.T) {
	// 有效的结构体
	user := TestUser{
		Name:  "李四",
		Email: "lisi@example.com",
		Age:   30,
	}
	err := validate.Validate.Struct(user)
	if err != nil {
		t.Fatalf("全局 Validate 验证有效结构体应通过，但返回了错误: %v", err)
	}

	// 无效的结构体
	invalid := TestUser{
		Name:  "",
		Email: "invalid-email",
		Age:   10,
	}
	err = validate.Validate.Struct(invalid)
	if err == nil {
		t.Fatal("全局 Validate 验证无效结构体应返回错误")
	}
}

// ======================== 7. IsValidationError ========================

func TestIsValidationError_True(t *testing.T) {
	v := validate.NewValidator()
	user := TestUserOnlyRequired{}
	err := v.ValidateStruct(user)
	if err == nil {
		t.Fatal("应返回验证错误")
	}

	if !validate.IsValidationError(err) {
		t.Error("验证错误应返回 true")
	}
}

func TestIsValidationError_False(t *testing.T) {
	plainErr := errors.New("这是一个普通错误")
	if validate.IsValidationError(plainErr) {
		t.Error("普通错误应返回 false")
	}

	if validate.IsValidationError(nil) {
		t.Error("nil 错误应返回 false")
	}
}

// ======================== 8. FirstErrorMessage / AllErrorMessages ========================

func TestFirstErrorMessage(t *testing.T) {
	v := validate.NewValidator()
	user := TestUserOnlyRequired{}
	err := v.ValidateStruct(user)
	if err == nil {
		t.Fatal("应返回验证错误")
	}

	msg := validate.FirstErrorMessage(err)
	if msg != "Name为必填字段" {
		t.Errorf("第一条错误消息应为'Name为必填字段'，实际: %s", msg)
	}
}

func TestAllErrorMessages(t *testing.T) {
	v := validate.NewValidator()
	user := TestUserWithMessages{}
	err := v.ValidateStruct(user)
	if err == nil {
		t.Fatal("应返回验证错误")
	}

	msg := validate.AllErrorMessages(err)
	if !strings.Contains(msg, "; ") {
		t.Errorf("错误消息应以分号空格分隔，实际: %s", msg)
	}
}

func TestFirstErrorMessage_NonValidationError(t *testing.T) {
	plainErr := errors.New("这是一个普通错误")
	msg := validate.FirstErrorMessage(plainErr)
	if msg != "这是一个普通错误" {
		t.Errorf("非验证错误应返回原始错误消息，实际: %s", msg)
	}

	// nil 错误
	msg = validate.FirstErrorMessage(nil)
	if msg != "" {
		t.Errorf("nil 错误应返回空字符串，实际: %s", msg)
	}
}

func TestAllErrorMessages_NonValidationError(t *testing.T) {
	plainErr := errors.New("这是一个普通错误")
	msg := validate.AllErrorMessages(plainErr)
	if msg != "这是一个普通错误" {
		t.Errorf("非验证错误应返回原始错误消息，实际: %s", msg)
	}

	// nil 错误
	msg = validate.AllErrorMessages(nil)
	if msg != "" {
		t.Errorf("nil 错误应返回空字符串，实际: %s", msg)
	}
}

// ======================== 9. getDefaultErrorMessage 各种 tag 覆盖 ========================

func TestDefaultErrorMessage_Required(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagRequired{Name: ""})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "Name为必填字段" {
		t.Errorf("required 错误消息应为'Name为必填字段'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_Email(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagEmail{Email: "not-an-email"})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "Email必须是有效的邮箱地址" {
		t.Errorf("email 错误消息应为'Email必须是有效的邮箱地址'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_URL(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagURL{URL: "not-a-url"})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "URL必须是有效的URL地址" {
		t.Errorf("url 错误消息应为'URL必须是有效的URL地址'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_MinString(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagMinString{Name: "ab"})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "Name长度不能小于3" {
		t.Errorf("min 字符串错误消息应为'Name长度不能小于3'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_MinInt(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagMinInt{Age: 10})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "Age不能小于18" {
		t.Errorf("min 整数错误消息应为'Age不能小于18'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_MaxString(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagMaxString{Name: "this-name-is-too-long"})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "Name长度不能大于10" {
		t.Errorf("max 字符串错误消息应为'Name长度不能大于10'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_MaxInt(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagMaxInt{Age: 200})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "Age不能大于120" {
		t.Errorf("max 整数错误消息应为'Age不能大于120'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_Len(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagLen{Code: "12345"})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "Code长度必须为6" {
		t.Errorf("len 错误消息应为'Code长度必须为6'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_Oneof(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagOneof{Status: "pending"})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "Status必须是以下之一：active inactive" {
		t.Errorf("oneof 错误消息应为'Status必须是以下之一：active inactive'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_IP(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagIP{IP: "not-an-ip"})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "IP必须是有效的IP地址" {
		t.Errorf("ip 错误消息应为'IP必须是有效的IP地址'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_UUID(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagUUID{ID: "not-a-uuid"})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "ID必须是有效的UUID" {
		t.Errorf("uuid 错误消息应为'ID必须是有效的UUID'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_Alpha(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagAlpha{Name: "hello123"})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "Name只能包含字母" {
		t.Errorf("alpha 错误消息应为'Name只能包含字母'，实际: %s", errs[0].Message)
	}
}

func TestDefaultErrorMessage_Numeric(t *testing.T) {
	v := validate.NewValidator()
	err := v.ValidateStruct(TestTagNumeric{Value: "abc"})
	if err == nil {
		t.Fatal("应返回验证错误")
	}
	errs := validate.FormatValidationErrors(err)
	if len(errs) == 0 {
		t.Fatal("应返回错误信息")
	}
	if errs[0].Message != "Value必须是数字" {
		t.Errorf("numeric 错误消息应为'Value必须是数字'，实际: %s", errs[0].Message)
	}
}

// ======================== 辅助类型测试 ========================

func TestValidationError_Error(t *testing.T) {
	ve := validate.ValidationError{
		Field:   "Name",
		Tag:     "required",
		Message: "Name为必填字段",
	}
	if ve.Error() != "Name为必填字段" {
		t.Errorf("ValidationError.Error() 应返回 Message 字段值，实际: %s", ve.Error())
	}
}

func TestValidationErrors_Error(t *testing.T) {
	ves := validate.ValidationErrors{
		{Field: "Name", Tag: "required", Message: "Name为必填字段"},
		{Field: "Email", Tag: "email", Message: "Email必须是有效的邮箱地址"},
	}
	errMsg := ves.Error()
	if !strings.Contains(errMsg, "Name为必填字段") {
		t.Errorf("ValidationErrors.Error() 应包含第一条消息，实际: %s", errMsg)
	}
	if !strings.Contains(errMsg, "Email必须是有效的邮箱地址") {
		t.Errorf("ValidationErrors.Error() 应包含第二条消息，实际: %s", errMsg)
	}
	if !strings.Contains(errMsg, "; ") {
		t.Errorf("多条错误消息应以分号空格分隔，实际: %s", errMsg)
	}
}
