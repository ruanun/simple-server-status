import axios from "axios"
import type {AxiosError, AxiosInstance, AxiosRequestConfig, AxiosResponse} from "axios";
import {message} from "ant-design-vue";
import {checkStatus} from "@/api/helper/checkStatus";


// * 请求响应参数(不包含data)
export interface Result {
    code: string;
    msg: string;
}

// * 请求响应参数(包含data)
export interface ResultData<T = any> extends Result {
    data: T;
}

/**
 * @description：请求配置
 */
export enum ResultEnum {
    SUCCESS = 200,
    ERROR = 500,
    OVERDUE = 401,
    TIMEOUT = 30000,
    TYPE = "success"
}

/**
 * @description：请求方法
 */
export enum RequestEnum {
    GET = "GET",
    POST = "POST",
    PATCH = "PATCH",
    PUT = "PUT",
    DELETE = "DELETE"
}

/**
 * @description：常用的contentTyp类型
 */
export enum ContentTypeEnum {
    // json
    JSON = "application/json;charset=UTF-8",
    // text
    TEXT = "text/plain;charset=UTF-8",
    // form-data 一般配合qs
    FORM_URLENCODED = "application/x-www-form-urlencoded;charset=UTF-8",
    // form-data 上传
    FORM_DATA = "multipart/form-data;charset=UTF-8"
}

const config = {
    // 默认地址请求地址，可在 .env.*** 文件中修改
    baseURL: import.meta.env.VITE_BASE_URL as string,
    // 设置超时时间（30s）
    timeout: ResultEnum.TIMEOUT as number,
    // 跨域时候允许携带凭证
    withCredentials: true
};

// console.log("import.meta.env  ", import.meta.env.MODE)

class RequestHttp {
    service: AxiosInstance;

    public constructor(config: AxiosRequestConfig) {
        // 实例化axios
        this.service = axios.create(config);

        /**
         * @description 响应拦截器
         *  服务器换返回信息 -> [拦截统一处理] -> 客户端JS获取到信息
         */
        this.service.interceptors.response.use(
            (response: AxiosResponse) => {
                const {data} = response;
                if (data.code && data.code !== ResultEnum.SUCCESS) {
                    message.error(data.msg);
                    return Promise.reject(data);
                }
                return data;
            },
            async (error: AxiosError) => {
                const {response} = error;
                // 请求超时 && 网络错误单独判断，没有 response
                if (error.message.indexOf("timeout") !== -1) message.error("请求超时！请您稍后重试");
                if (error.message.indexOf("Network Error") !== -1) message.error("网络错误！请您稍后重试");
                // 根据响应的错误状态码，做不同的处理
                if (response) checkStatus(response.status);
                // 服务器结果都没有返回(可能服务器错误可能客户端断网)，断网处理:可以跳转到断网页面
                // if (!window.navigator.onLine) router.replace("/500");
                return Promise.reject(error);
            }
        );
    }

    // * 常用请求方法封装
    get<T>(url: string, params?: object, _object = {}): Promise<ResultData<T>> {
        return this.service.get(url, {params, ..._object});
    }

    post<T>(url: string, params?: object, _object = {}): Promise<ResultData<T>> {
        return this.service.post(url, params, _object);
    }

    put<T>(url: string, params?: object, _object = {}): Promise<ResultData<T>> {
        return this.service.put(url, params, _object);
    }

    delete<T>(url: string, params?: any, _object = {}): Promise<ResultData<T>> {
        return this.service.delete(url, {params, ..._object});
    }

    download(url: string, params?: object, _object = {}): Promise<BlobPart> {
        return this.service.post(url, params, {..._object, responseType: "blob"});
    }
}

export default new RequestHttp(config);
