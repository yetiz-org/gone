package goai

import "reflect"

// _PreparedSet is the immutable-after-prepare metadata used by sequential
// and parallel assemblers. Example values hanging off Spec/OperationDoc
// graphs are caller-owned and must not be mutated during generation.
type _PreparedSet struct {
	Operations  []_PreparedOperation
	SchemaNames map[reflect.Type]string
}

// _PreparedOperation is one walker candidate with provider, docstring,
// registry, and schema-name metadata already resolved. Assemblers must
// use an owned clone; they must not invoke Handler, Acceptance, Spec,
// docstring, or SchemaNameProvider callbacks.
type _PreparedOperation struct {
	Candidate              OperationCandidate
	ResolvedProfiles       []string
	Spec                   Spec
	HasRegistryEntry       bool
	ReqType                reflect.Type
	RespType               reflect.Type
	Tags                   []string
	SecurityRefs           []SecurityRef
	OperationDoc           *OperationDoc
	OperationDocOp         *Operation
	OperationDocMatches    bool
	PackagePath            string
	SynthesizedOperationID string
}

func _PrepareOperationCandidates(candidates []OperationCandidate, profile *Profile, opts BuildOptions) (prepared _PreparedSet) {
	classifier := opts.Classifier
	if classifier == nil {
		classifier = BuiltinClassify
	}

	tsClassifier := opts.TagSecurityClassifier
	if tsClassifier == nil {
		tsClassifier = DefaultClassifier()
	}

	docExtractor := cachedOperationDocExtractor(opts.OperationDocExtractor)
	groupCounts := _CountCandidateGroups(candidates)
	operations := make([]_PreparedOperation, 0, len(candidates))
	matching := make([]_PreparedOperation, 0, len(candidates))

	for _, c := range candidates {
		soleCandidate := groupCounts[_CandidateGroupKeyFor(c)] == 1
		for _, ec := range _ExpandCandidateFromDocstringEndpoints(c, docExtractor, soleCandidate) {
			ec = _CandidateWithPathTemplateParams(ec)
			profiles := ec.Profiles
			if len(profiles) == 0 {
				profiles = classifier(ec)
			}

			spec, hasEntry, reqType, respType := _ResolveCandidateSpec(ec)
			operationDoc, operationDocOp, operationDocMatches := _ResolveCandidateOperationDoc(ec, docExtractor)
			tags := _ResolvedOperationTags(spec, operationDocOp, ec, tsClassifier)
			packagePath := handlerPackagePath(ec.Handler)
			matches := profile == nil || profile.matches(ec.Path, packagePath, profiles, tags)
			securityRefs := append([]SecurityRef(nil), ec.SecurityRefs...)
			if matches {
				for _, ref := range spec.Security() {
					securityRefs = append(securityRefs, ref)
				}

				securityRefs = dedupeSecurityRefs(securityRefs)
				if len(securityRefs) == 0 && tsClassifier != nil {
					securityRefs = tsClassifier.ClassifySecurity(ec.Acceptances)
				}
			}

			candidate := _CloneOperationCandidate(ec)
			candidate.Handler = nil
			candidate.Acceptances = nil
			op := _PreparedOperation{
				Candidate:              candidate,
				ResolvedProfiles:       append([]string(nil), profiles...),
				Spec:                   _CloneSpec(spec),
				HasRegistryEntry:       hasEntry,
				ReqType:                reqType,
				RespType:               respType,
				Tags:                   append([]string(nil), tags...),
				SecurityRefs:           _CloneSecurityRefs(securityRefs),
				OperationDoc:           _CloneOperationDoc(operationDoc),
				OperationDocOp:         _CloneOperationPtr(operationDocOp),
				OperationDocMatches:    operationDocMatches,
				PackagePath:            packagePath,
				SynthesizedOperationID: synthesiseOperationID(ec),
			}
			operations = append(operations, op)
			if matches {
				matching = append(matching, op)
			}
		}
	}

	prepared.Operations = operations
	prepared.SchemaNames = _ResolveSchemaNames(matching)

	return prepared
}

func _ResolveCandidateSpec(c OperationCandidate) (spec Spec, hasEntry bool, reqType reflect.Type, respType reflect.Type) {
	entry, ok := Lookup(c.Handler, c.Method)
	if ok {
		return entry.Spec, true, entry.ReqType, entry.RespType
	}

	if base, ok := c.Handler.(SpecProvider); ok {
		spec = base.GOAISpec()
	}

	if perMethod, ok := perMethodSpec(c.Handler, c.HandlerMethod); ok {
		spec = perMethod
	}

	return spec, false, nil, nil
}

func _ResolveCandidateOperationDoc(c OperationCandidate, docExtractor OperationDocExtractor) (doc *OperationDoc, docOp *Operation, matches bool) {
	if docExtractor == nil {
		return nil, nil, false
	}

	extracted, ok := docExtractor(c.Handler, c.HandlerMethod)
	if !ok {
		return nil, nil, false
	}

	return extracted, _OperationDocOperationForCandidate(extracted, c), _OperationDocMatches(extracted, c)
}

func _ResolvedOperationTags(spec Spec, operationDocOp *Operation, c OperationCandidate, classifier *Classifier) (tags []string) {
	tags = _DeduplicateStrings(spec.Tags())
	if len(tags) == 0 && operationDocOp != nil {
		tags = _DeduplicateStrings(operationDocOp.Tags)
	}

	if len(tags) == 0 && classifier != nil {
		if tag := classifier.ClassifyTag(c.Path); tag != "" {
			tags = []string{tag}
		}
	}

	return tags
}

func _ResolveSchemaNames(operations []_PreparedOperation) (names map[reflect.Type]string) {
	builder := newSchemaBuilder(NewComponents())
	for _, op := range operations {
		if op.ReqType != nil {
			builder.build(op.ReqType)
		}

		if op.RespType != nil {
			builder.build(op.RespType)
		}

		for _, rs := range op.Spec.Responses() {
			if rs == nil || rs.SchemaType == nil {
				continue
			}

			builder.build(rs.SchemaType)
		}
	}

	return builder.resolvedSchemaNames
}

func _ClonePreparedSet(set _PreparedSet) (cloned _PreparedSet) {
	cloned.SchemaNames = _CloneTypeNameMap(set.SchemaNames)
	if len(set.Operations) == 0 {
		return cloned
	}

	cloned.Operations = make([]_PreparedOperation, len(set.Operations))
	for i, op := range set.Operations {
		cloned.Operations[i] = _ClonePreparedOperation(op)
	}

	return cloned
}

func _ClonePreparedOperation(op _PreparedOperation) (cloned _PreparedOperation) {
	cloned = op
	cloned.Candidate = _CloneOperationCandidate(op.Candidate)
	cloned.ResolvedProfiles = append([]string(nil), op.ResolvedProfiles...)
	cloned.Spec = _CloneSpec(op.Spec)
	cloned.Tags = append([]string(nil), op.Tags...)
	cloned.SecurityRefs = _CloneSecurityRefs(op.SecurityRefs)
	cloned.OperationDoc = _CloneOperationDoc(op.OperationDoc)
	cloned.OperationDocOp = _CloneOperationPtr(op.OperationDocOp)

	return cloned
}

func _CloneOperationCandidate(candidate OperationCandidate) (cloned OperationCandidate) {
	cloned = candidate
	cloned.PathParams = append([]PathParam(nil), candidate.PathParams...)
	cloned.Profiles = append([]string(nil), candidate.Profiles...)
	cloned.SecurityRefs = _CloneSecurityRefs(candidate.SecurityRefs)
	if candidate.Acceptances != nil {
		cloned.Acceptances = append(candidate.Acceptances[:0:0], candidate.Acceptances...)
	}

	return cloned
}

func _CloneTypeNameMap(names map[reflect.Type]string) (cloned map[reflect.Type]string) {
	if names == nil {
		return nil
	}

	cloned = make(map[reflect.Type]string, len(names))
	for key, value := range names {
		cloned[key] = value
	}

	return cloned
}

func _CloneSpec(spec Spec) (cloned Spec) {
	cloned = spec
	cloned.tags = append([]string(nil), spec.tags...)
	cloned.examples = cloneAnyMap(spec.examples)
	cloned.multiExamples = _CloneMultiExamples(spec.multiExamples)
	cloned.security = _CloneSecurityRefs(spec.security)
	cloned.extraHeaders = append([]HeaderDef(nil), spec.extraHeaders...)
	cloned.extraParams = append([]PathParam(nil), spec.extraParams...)
	cloned.externalDocs = _CloneExternalDocs(spec.externalDocs)
	cloned.callbacks = _CloneCallbacks(spec.callbacks)
	cloned.servers = _CloneServers(spec.servers)
	cloned.responses = _CloneResponseSpecs(spec.responses)

	return cloned
}

func _CloneSecurityRefs(refs []SecurityRef) (cloned []SecurityRef) {
	if refs == nil {
		return nil
	}

	cloned = make([]SecurityRef, len(refs))
	for i, ref := range refs {
		cloned[i] = SecurityRef{
			Scheme: ref.Scheme,
			Scopes: append([]string(nil), ref.Scopes...),
		}
	}

	return cloned
}

func _CloneMultiExamples(examples map[string]map[string]*Example) (cloned map[string]map[string]*Example) {
	if examples == nil {
		return nil
	}

	cloned = make(map[string]map[string]*Example, len(examples))
	for mediaType, bucket := range examples {
		cloned[mediaType] = _CloneExampleMap(bucket)
	}

	return cloned
}

func _CloneResponseSpecs(responses map[string]*ResponseSpec) (cloned map[string]*ResponseSpec) {
	if responses == nil {
		return nil
	}

	cloned = make(map[string]*ResponseSpec, len(responses))
	for status, rs := range responses {
		cloned[status] = _CloneResponseSpec(rs)
	}

	return cloned
}

func _CloneResponseSpec(rs *ResponseSpec) (cloned *ResponseSpec) {
	if rs == nil {
		return nil
	}

	copySpec := *rs
	copySpec.Schema = cloneSchema(rs.Schema)
	copySpec.Examples = _CloneExampleMap(rs.Examples)
	copySpec.Headers = append([]HeaderDef(nil), rs.Headers...)
	cloned = &copySpec

	return cloned
}

func _CloneOperationDoc(doc *OperationDoc) (cloned *OperationDoc) {
	if doc == nil {
		return nil
	}

	copyDoc := *doc
	copyDoc.Endpoints = append([]OperationEndpoint(nil), doc.Endpoints...)
	copyDoc.Operation = _CloneOperationDeep(doc.Operation)
	copyDoc.Schemas = cloneSchemaMap(doc.Schemas)
	copyDoc.SchemaPackages = _CloneSchemaPackageMap(doc.SchemaPackages)
	if len(doc.EndpointOperations) > 0 {
		copyDoc.EndpointOperations = make([]OperationEndpointDoc, len(doc.EndpointOperations))
		for i, item := range doc.EndpointOperations {
			copyDoc.EndpointOperations[i] = OperationEndpointDoc{
				Endpoint:       item.Endpoint,
				Operation:      _CloneOperationDeep(item.Operation),
				Schemas:        cloneSchemaMap(item.Schemas),
				SchemaPackages: _CloneSchemaPackageMap(item.SchemaPackages),
			}
		}
	}

	cloned = &copyDoc

	return cloned
}

func _CloneOperationPtr(op *Operation) (cloned *Operation) {
	if op == nil {
		return nil
	}

	copyOp := _CloneOperationDeep(*op)
	cloned = &copyOp

	return cloned
}

func _CloneOperationDeep(op Operation) (cloned Operation) {
	cloned = op
	cloned.Tags = append([]string(nil), op.Tags...)
	cloned.ExternalDocs = _CloneExternalDocs(op.ExternalDocs)
	cloned.Parameters = _CloneParameters(op.Parameters)
	cloned.RequestBody = _CloneRequestBody(op.RequestBody)
	cloned.Responses = _CloneResponses(op.Responses)
	cloned.Callbacks = _CloneCallbacks(op.Callbacks)
	cloned.Security = _CloneSecurityRequirements(op.Security)
	cloned.Servers = _CloneServers(op.Servers)
	cloned.Extensions = cloneAnyMap(op.Extensions)

	return cloned
}

func _ClonePathItem(item *PathItem) (cloned *PathItem) {
	if item == nil {
		return nil
	}

	copyItem := *item
	copyItem.Get = _CloneOperationPtr(item.Get)
	copyItem.Put = _CloneOperationPtr(item.Put)
	copyItem.Post = _CloneOperationPtr(item.Post)
	copyItem.Delete = _CloneOperationPtr(item.Delete)
	copyItem.Options = _CloneOperationPtr(item.Options)
	copyItem.Head = _CloneOperationPtr(item.Head)
	copyItem.Patch = _CloneOperationPtr(item.Patch)
	copyItem.Trace = _CloneOperationPtr(item.Trace)
	copyItem.Servers = _CloneServers(item.Servers)
	copyItem.Parameters = _CloneParameters(item.Parameters)
	copyItem.Extensions = cloneAnyMap(item.Extensions)
	cloned = &copyItem

	return cloned
}

func _CloneParameters(params []*Parameter) (cloned []*Parameter) {
	if params == nil {
		return nil
	}

	cloned = make([]*Parameter, len(params))
	for i, param := range params {
		cloned[i] = _CloneParameter(param)
	}

	return cloned
}

func _CloneParameter(param *Parameter) (cloned *Parameter) {
	if param == nil {
		return nil
	}

	copyParam := *param
	copyParam.Explode = _CloneBoolPtr(param.Explode)
	copyParam.Schema = cloneSchema(param.Schema)
	copyParam.Examples = _CloneExampleMap(param.Examples)
	copyParam.Content = _CloneMediaTypes(param.Content)
	copyParam.Extensions = cloneAnyMap(param.Extensions)
	cloned = &copyParam

	return cloned
}

func _CloneRequestBody(body *RequestBody) (cloned *RequestBody) {
	if body == nil {
		return nil
	}

	copyBody := *body
	copyBody.Content = _CloneMediaTypes(body.Content)
	copyBody.Extensions = cloneAnyMap(body.Extensions)
	cloned = &copyBody

	return cloned
}

func _CloneResponses(responses map[string]*Response) (cloned map[string]*Response) {
	if responses == nil {
		return nil
	}

	cloned = make(map[string]*Response, len(responses))
	for status, resp := range responses {
		cloned[status] = _CloneResponse(resp)
	}

	return cloned
}

func _CloneResponse(resp *Response) (cloned *Response) {
	if resp == nil {
		return nil
	}

	copyResp := *resp
	copyResp.Headers = _CloneHeaders(resp.Headers)
	copyResp.Content = _CloneMediaTypes(resp.Content)
	copyResp.Links = _CloneLinks(resp.Links)
	copyResp.Extensions = cloneAnyMap(resp.Extensions)
	cloned = &copyResp

	return cloned
}

func _CloneHeaders(headers map[string]*Header) (cloned map[string]*Header) {
	if headers == nil {
		return nil
	}

	cloned = make(map[string]*Header, len(headers))
	for name, header := range headers {
		cloned[name] = _CloneHeader(header)
	}

	return cloned
}

func _CloneHeader(header *Header) (cloned *Header) {
	if header == nil {
		return nil
	}

	copyHeader := *header
	copyHeader.Explode = _CloneBoolPtr(header.Explode)
	copyHeader.Schema = cloneSchema(header.Schema)
	copyHeader.Examples = _CloneExampleMap(header.Examples)
	copyHeader.Content = _CloneMediaTypes(header.Content)
	copyHeader.Extensions = cloneAnyMap(header.Extensions)
	cloned = &copyHeader

	return cloned
}

func _CloneMediaTypes(content map[string]*MediaType) (cloned map[string]*MediaType) {
	if content == nil {
		return nil
	}

	cloned = make(map[string]*MediaType, len(content))
	for mediaType, mt := range content {
		cloned[mediaType] = _CloneMediaType(mt)
	}

	return cloned
}

func _CloneMediaType(mt *MediaType) (cloned *MediaType) {
	if mt == nil {
		return nil
	}

	copyMT := *mt
	copyMT.Schema = cloneSchema(mt.Schema)
	copyMT.Examples = _CloneExampleMap(mt.Examples)
	copyMT.Encoding = _CloneEncodings(mt.Encoding)
	copyMT.Extensions = cloneAnyMap(mt.Extensions)
	cloned = &copyMT

	return cloned
}

func _CloneEncodings(encodings map[string]*Encoding) (cloned map[string]*Encoding) {
	if encodings == nil {
		return nil
	}

	cloned = make(map[string]*Encoding, len(encodings))
	for name, encoding := range encodings {
		if encoding == nil {
			cloned[name] = nil

			continue
		}

		copyEncoding := *encoding
		copyEncoding.Headers = _CloneHeaders(encoding.Headers)
		copyEncoding.Explode = _CloneBoolPtr(encoding.Explode)
		copyEncoding.Extensions = cloneAnyMap(encoding.Extensions)
		cloned[name] = &copyEncoding
	}

	return cloned
}

func _CloneExampleMap(examples map[string]*Example) (cloned map[string]*Example) {
	if examples == nil {
		return nil
	}

	cloned = make(map[string]*Example, len(examples))
	for name, example := range examples {
		cloned[name] = _CloneExample(example)
	}

	return cloned
}

func _CloneExample(example *Example) (cloned *Example) {
	if example == nil {
		return nil
	}

	copyExample := *example
	copyExample.Extensions = cloneAnyMap(example.Extensions)
	cloned = &copyExample

	return cloned
}

func _CloneCallbacks(callbacks map[string]Callback) (cloned map[string]Callback) {
	if callbacks == nil {
		return nil
	}

	cloned = make(map[string]Callback, len(callbacks))
	for name, callback := range callbacks {
		cloned[name] = _CloneCallback(callback)
	}

	return cloned
}

func _CloneCallback(callback Callback) (cloned Callback) {
	if callback == nil {
		return nil
	}

	cloned = make(Callback, len(callback))
	for expr, item := range callback {
		cloned[expr] = _ClonePathItem(item)
	}

	return cloned
}

func _CloneLinks(links map[string]*Link) (cloned map[string]*Link) {
	if links == nil {
		return nil
	}

	cloned = make(map[string]*Link, len(links))
	for name, link := range links {
		if link == nil {
			cloned[name] = nil

			continue
		}

		copyLink := *link
		copyLink.Parameters = cloneAnyMap(link.Parameters)
		if link.Server != nil {
			server := _CloneServer(*link.Server)
			copyLink.Server = &server
		}

		copyLink.Extensions = cloneAnyMap(link.Extensions)
		cloned[name] = &copyLink
	}

	return cloned
}

func _CloneServers(servers []Server) (cloned []Server) {
	if servers == nil {
		return nil
	}

	cloned = make([]Server, len(servers))
	for i, server := range servers {
		cloned[i] = _CloneServer(server)
	}

	return cloned
}

func _CloneServer(server Server) (cloned Server) {
	cloned = server
	if server.Variables != nil {
		cloned.Variables = make(map[string]*ServerVariable, len(server.Variables))
		for name, variable := range server.Variables {
			if variable == nil {
				cloned.Variables[name] = nil

				continue
			}

			copyVar := *variable
			copyVar.Enum = append([]string(nil), variable.Enum...)
			copyVar.Extensions = cloneAnyMap(variable.Extensions)
			cloned.Variables[name] = &copyVar
		}
	}

	cloned.Extensions = cloneAnyMap(server.Extensions)

	return cloned
}

func _CloneExternalDocs(docs *ExternalDocumentation) (cloned *ExternalDocumentation) {
	if docs == nil {
		return nil
	}

	copyDocs := *docs
	copyDocs.Extensions = cloneAnyMap(docs.Extensions)
	cloned = &copyDocs

	return cloned
}

func _CloneSecurityRequirements(security *[]map[string][]string) (cloned *[]map[string][]string) {
	if security == nil {
		return nil
	}

	copyReqs := make([]map[string][]string, len(*security))
	for i, req := range *security {
		if req == nil {
			continue
		}

		copyReq := make(map[string][]string, len(req))
		for scheme, scopes := range req {
			copyReq[scheme] = append([]string(nil), scopes...)
		}

		copyReqs[i] = copyReq
	}

	cloned = &copyReqs

	return cloned
}

func _CloneBoolPtr(value *bool) (cloned *bool) {
	if value == nil {
		return nil
	}

	copyValue := *value
	cloned = &copyValue

	return cloned
}
