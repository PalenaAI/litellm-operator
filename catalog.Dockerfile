FROM quay.io/operator-framework/opm@sha256:f9ecca9c1fd94862b39877a3da4a020986fa5fcb693fac626e969a7e2640267b

COPY catalog/configs /configs

LABEL operators.operatorframework.io.index.configs.v1=/configs
