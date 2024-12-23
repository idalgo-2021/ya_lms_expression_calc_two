// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0-devel
// 	protoc        v3.14.0
// source: grpc/proto_exchange/exchange.proto

package proto_exchange

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type EchoRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address   string `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	StartTime string `protobuf:"bytes,2,opt,name=startTime,proto3" json:"startTime,omitempty"`
	Capacity  int32  `protobuf:"varint,3,opt,name=capacity,proto3" json:"capacity,omitempty"`
	Queue     int32  `protobuf:"varint,4,opt,name=queue,proto3" json:"queue,omitempty"`
}

func (x *EchoRequest) Reset() {
	*x = EchoRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EchoRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EchoRequest) ProtoMessage() {}

func (x *EchoRequest) ProtoReflect() protoreflect.Message {
	mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EchoRequest.ProtoReflect.Descriptor instead.
func (*EchoRequest) Descriptor() ([]byte, []int) {
	return file_grpc_proto_exchange_exchange_proto_rawDescGZIP(), []int{0}
}

func (x *EchoRequest) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *EchoRequest) GetStartTime() string {
	if x != nil {
		return x.StartTime
	}
	return ""
}

func (x *EchoRequest) GetCapacity() int32 {
	if x != nil {
		return x.Capacity
	}
	return 0
}

func (x *EchoRequest) GetQueue() int32 {
	if x != nil {
		return x.Queue
	}
	return 0
}

type EchoResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Status string `protobuf:"bytes,1,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *EchoResponse) Reset() {
	*x = EchoResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EchoResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EchoResponse) ProtoMessage() {}

func (x *EchoResponse) ProtoReflect() protoreflect.Message {
	mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EchoResponse.ProtoReflect.Descriptor instead.
func (*EchoResponse) Descriptor() ([]byte, []int) {
	return file_grpc_proto_exchange_exchange_proto_rawDescGZIP(), []int{1}
}

func (x *EchoResponse) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

type ResultRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address       string  `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	RecordId      int64   `protobuf:"varint,2,opt,name=recordId,proto3" json:"recordId,omitempty"`
	TaskId        int64   `protobuf:"varint,3,opt,name=taskId,proto3" json:"taskId,omitempty"`
	IsWrong       bool    `protobuf:"varint,4,opt,name=isWrong,proto3" json:"isWrong,omitempty"`
	IsFinished    bool    `protobuf:"varint,5,opt,name=isFinished,proto3" json:"isFinished,omitempty"`
	Comment       string  `protobuf:"bytes,6,opt,name=comment,proto3" json:"comment,omitempty"`
	RpnExpression string  `protobuf:"bytes,7,opt,name=rpnExpression,proto3" json:"rpnExpression,omitempty"`
	Result        float64 `protobuf:"fixed64,8,opt,name=result,proto3" json:"result,omitempty"`
}

func (x *ResultRequest) Reset() {
	*x = ResultRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ResultRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResultRequest) ProtoMessage() {}

func (x *ResultRequest) ProtoReflect() protoreflect.Message {
	mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResultRequest.ProtoReflect.Descriptor instead.
func (*ResultRequest) Descriptor() ([]byte, []int) {
	return file_grpc_proto_exchange_exchange_proto_rawDescGZIP(), []int{2}
}

func (x *ResultRequest) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *ResultRequest) GetRecordId() int64 {
	if x != nil {
		return x.RecordId
	}
	return 0
}

func (x *ResultRequest) GetTaskId() int64 {
	if x != nil {
		return x.TaskId
	}
	return 0
}

func (x *ResultRequest) GetIsWrong() bool {
	if x != nil {
		return x.IsWrong
	}
	return false
}

func (x *ResultRequest) GetIsFinished() bool {
	if x != nil {
		return x.IsFinished
	}
	return false
}

func (x *ResultRequest) GetComment() string {
	if x != nil {
		return x.Comment
	}
	return ""
}

func (x *ResultRequest) GetRpnExpression() string {
	if x != nil {
		return x.RpnExpression
	}
	return ""
}

func (x *ResultRequest) GetResult() float64 {
	if x != nil {
		return x.Result
	}
	return 0
}

type ResultResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Status string `protobuf:"bytes,1,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *ResultResponse) Reset() {
	*x = ResultResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ResultResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResultResponse) ProtoMessage() {}

func (x *ResultResponse) ProtoReflect() protoreflect.Message {
	mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResultResponse.ProtoReflect.Descriptor instead.
func (*ResultResponse) Descriptor() ([]byte, []int) {
	return file_grpc_proto_exchange_exchange_proto_rawDescGZIP(), []int{3}
}

func (x *ResultResponse) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

type AddTaskRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RecordId      int64  `protobuf:"varint,1,opt,name=recordId,proto3" json:"recordId,omitempty"`
	TaskId        int64  `protobuf:"varint,2,opt,name=taskId,proto3" json:"taskId,omitempty"`
	Expression    string `protobuf:"bytes,3,opt,name=Expression,proto3" json:"Expression,omitempty"`
	RpnExpression string `protobuf:"bytes,4,opt,name=rpnExpression,proto3" json:"rpnExpression,omitempty"`
}

func (x *AddTaskRequest) Reset() {
	*x = AddTaskRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AddTaskRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AddTaskRequest) ProtoMessage() {}

func (x *AddTaskRequest) ProtoReflect() protoreflect.Message {
	mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AddTaskRequest.ProtoReflect.Descriptor instead.
func (*AddTaskRequest) Descriptor() ([]byte, []int) {
	return file_grpc_proto_exchange_exchange_proto_rawDescGZIP(), []int{4}
}

func (x *AddTaskRequest) GetRecordId() int64 {
	if x != nil {
		return x.RecordId
	}
	return 0
}

func (x *AddTaskRequest) GetTaskId() int64 {
	if x != nil {
		return x.TaskId
	}
	return 0
}

func (x *AddTaskRequest) GetExpression() string {
	if x != nil {
		return x.Expression
	}
	return ""
}

func (x *AddTaskRequest) GetRpnExpression() string {
	if x != nil {
		return x.RpnExpression
	}
	return ""
}

type AddTaskResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Status string `protobuf:"bytes,1,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *AddTaskResponse) Reset() {
	*x = AddTaskResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AddTaskResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AddTaskResponse) ProtoMessage() {}

func (x *AddTaskResponse) ProtoReflect() protoreflect.Message {
	mi := &file_grpc_proto_exchange_exchange_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AddTaskResponse.ProtoReflect.Descriptor instead.
func (*AddTaskResponse) Descriptor() ([]byte, []int) {
	return file_grpc_proto_exchange_exchange_proto_rawDescGZIP(), []int{5}
}

func (x *AddTaskResponse) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

var File_grpc_proto_exchange_exchange_proto protoreflect.FileDescriptor

var file_grpc_proto_exchange_exchange_proto_rawDesc = []byte{
	0x0a, 0x22, 0x67, 0x72, 0x70, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x65, 0x78, 0x63,
	0x68, 0x61, 0x6e, 0x67, 0x65, 0x2f, 0x65, 0x78, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x65, 0x78, 0x63, 0x68,
	0x61, 0x6e, 0x67, 0x65, 0x22, 0x77, 0x0a, 0x0b, 0x45, 0x63, 0x68, 0x6f, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x1c, 0x0a,
	0x09, 0x73, 0x74, 0x61, 0x72, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x09, 0x73, 0x74, 0x61, 0x72, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x63,
	0x61, 0x70, 0x61, 0x63, 0x69, 0x74, 0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x63,
	0x61, 0x70, 0x61, 0x63, 0x69, 0x74, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x71, 0x75, 0x65, 0x75, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x05, 0x71, 0x75, 0x65, 0x75, 0x65, 0x22, 0x26, 0x0a,
	0x0c, 0x45, 0x63, 0x68, 0x6f, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x16, 0x0a,
	0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73,
	0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0xef, 0x01, 0x0a, 0x0d, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65,
	0x73, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73,
	0x73, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x49, 0x64, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x08, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x49, 0x64, 0x12, 0x16, 0x0a,
	0x06, 0x74, 0x61, 0x73, 0x6b, 0x49, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x06, 0x74,
	0x61, 0x73, 0x6b, 0x49, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x69, 0x73, 0x57, 0x72, 0x6f, 0x6e, 0x67,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x69, 0x73, 0x57, 0x72, 0x6f, 0x6e, 0x67, 0x12,
	0x1e, 0x0a, 0x0a, 0x69, 0x73, 0x46, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x65, 0x64, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x0a, 0x69, 0x73, 0x46, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x65, 0x64, 0x12,
	0x18, 0x0a, 0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x24, 0x0a, 0x0d, 0x72, 0x70, 0x6e,
	0x45, 0x78, 0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0d, 0x72, 0x70, 0x6e, 0x45, 0x78, 0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12,
	0x16, 0x0a, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x18, 0x08, 0x20, 0x01, 0x28, 0x01, 0x52,
	0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x22, 0x28, 0x0a, 0x0e, 0x52, 0x65, 0x73, 0x75, 0x6c,
	0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75,
	0x73, 0x22, 0x8a, 0x01, 0x0a, 0x0e, 0x41, 0x64, 0x64, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x49, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x49, 0x64,
	0x12, 0x16, 0x0a, 0x06, 0x74, 0x61, 0x73, 0x6b, 0x49, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x06, 0x74, 0x61, 0x73, 0x6b, 0x49, 0x64, 0x12, 0x1e, 0x0a, 0x0a, 0x45, 0x78, 0x70, 0x72,
	0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x45, 0x78,
	0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x24, 0x0a, 0x0d, 0x72, 0x70, 0x6e, 0x45,
	0x78, 0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0d, 0x72, 0x70, 0x6e, 0x45, 0x78, 0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x22, 0x29,
	0x0a, 0x0f, 0x41, 0x64, 0x64, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x32, 0x99, 0x01, 0x0a, 0x0b, 0x43, 0x61,
	0x6c, 0x63, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x12, 0x41, 0x0a, 0x04, 0x45, 0x63, 0x68,
	0x6f, 0x12, 0x1b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x65, 0x78, 0x63, 0x68, 0x61, 0x6e,
	0x67, 0x65, 0x2e, 0x45, 0x63, 0x68, 0x6f, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1c,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x65, 0x78, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x2e,
	0x45, 0x63, 0x68, 0x6f, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x47, 0x0a, 0x06,
	0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x1d, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x65,
	0x78, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x2e, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x65, 0x78,
	0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x2e, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0x58, 0x0a, 0x0a, 0x43, 0x61, 0x6c, 0x63, 0x44, 0x61, 0x65,
	0x6d, 0x6f, 0x6e, 0x12, 0x4a, 0x0a, 0x07, 0x41, 0x64, 0x64, 0x54, 0x61, 0x73, 0x6b, 0x12, 0x1e,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x65, 0x78, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x2e,
	0x41, 0x64, 0x64, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1f,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x65, 0x78, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x2e,
	0x41, 0x64, 0x64, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42,
	0x30, 0x5a, 0x2e, 0x79, 0x61, 0x5f, 0x6c, 0x6d, 0x73, 0x5f, 0x65, 0x78, 0x70, 0x72, 0x65, 0x73,
	0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x63, 0x61, 0x6c, 0x63, 0x5f, 0x74, 0x77, 0x6f, 0x2f, 0x67, 0x72,
	0x70, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5f, 0x65, 0x78, 0x63, 0x68, 0x61, 0x6e, 0x67,
	0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_grpc_proto_exchange_exchange_proto_rawDescOnce sync.Once
	file_grpc_proto_exchange_exchange_proto_rawDescData = file_grpc_proto_exchange_exchange_proto_rawDesc
)

func file_grpc_proto_exchange_exchange_proto_rawDescGZIP() []byte {
	file_grpc_proto_exchange_exchange_proto_rawDescOnce.Do(func() {
		file_grpc_proto_exchange_exchange_proto_rawDescData = protoimpl.X.CompressGZIP(file_grpc_proto_exchange_exchange_proto_rawDescData)
	})
	return file_grpc_proto_exchange_exchange_proto_rawDescData
}

var file_grpc_proto_exchange_exchange_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_grpc_proto_exchange_exchange_proto_goTypes = []interface{}{
	(*EchoRequest)(nil),     // 0: proto_exchange.EchoRequest
	(*EchoResponse)(nil),    // 1: proto_exchange.EchoResponse
	(*ResultRequest)(nil),   // 2: proto_exchange.ResultRequest
	(*ResultResponse)(nil),  // 3: proto_exchange.ResultResponse
	(*AddTaskRequest)(nil),  // 4: proto_exchange.AddTaskRequest
	(*AddTaskResponse)(nil), // 5: proto_exchange.AddTaskResponse
}
var file_grpc_proto_exchange_exchange_proto_depIdxs = []int32{
	0, // 0: proto_exchange.CalcManager.Echo:input_type -> proto_exchange.EchoRequest
	2, // 1: proto_exchange.CalcManager.Result:input_type -> proto_exchange.ResultRequest
	4, // 2: proto_exchange.CalcDaemon.AddTask:input_type -> proto_exchange.AddTaskRequest
	1, // 3: proto_exchange.CalcManager.Echo:output_type -> proto_exchange.EchoResponse
	3, // 4: proto_exchange.CalcManager.Result:output_type -> proto_exchange.ResultResponse
	5, // 5: proto_exchange.CalcDaemon.AddTask:output_type -> proto_exchange.AddTaskResponse
	3, // [3:6] is the sub-list for method output_type
	0, // [0:3] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_grpc_proto_exchange_exchange_proto_init() }
func file_grpc_proto_exchange_exchange_proto_init() {
	if File_grpc_proto_exchange_exchange_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_grpc_proto_exchange_exchange_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EchoRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpc_proto_exchange_exchange_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EchoResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpc_proto_exchange_exchange_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ResultRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpc_proto_exchange_exchange_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ResultResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpc_proto_exchange_exchange_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AddTaskRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_grpc_proto_exchange_exchange_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AddTaskResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_grpc_proto_exchange_exchange_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   2,
		},
		GoTypes:           file_grpc_proto_exchange_exchange_proto_goTypes,
		DependencyIndexes: file_grpc_proto_exchange_exchange_proto_depIdxs,
		MessageInfos:      file_grpc_proto_exchange_exchange_proto_msgTypes,
	}.Build()
	File_grpc_proto_exchange_exchange_proto = out.File
	file_grpc_proto_exchange_exchange_proto_rawDesc = nil
	file_grpc_proto_exchange_exchange_proto_goTypes = nil
	file_grpc_proto_exchange_exchange_proto_depIdxs = nil
}
